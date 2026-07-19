package snapshot

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Clawdlinux/agent-native-format/pkg/anf"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var fixtureNow = time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)

func TestCaptureComparesExactDocumentFactsAtCompletion(t *testing.T) {
	lister := newStaticLister(fixtureSource())
	result, err := Capture(context.Background(), lister, Options{
		Cluster:   "showcase-cluster",
		Namespace: "agentic-system",
		Clock: func() time.Time {
			if len(lister.calls) != 5 {
				t.Fatalf("clock called after %d lists, want 5", len(lister.calls))
			}
			return fixtureNow
		},
	})
	if err != nil {
		t.Fatalf("Capture returned error: %v", err)
	}

	if result.CapturedAt != fixtureNow {
		t.Fatalf("CapturedAt = %v, want %v", result.CapturedAt, fixtureNow)
	}
	if !strings.Contains(result.ANF, "@time 2026-07-14T12:00:00Z") {
		t.Fatalf("ANF does not use capture completion time: %q", result.ANF)
	}
	if len(result.View.Deployments) != 1 || len(result.View.Services) != 1 || len(result.View.Jobs) != 1 || len(result.View.CronJobs) != 1 {
		t.Fatalf("unexpected projected view: %#v", result.View)
	}
	if len(result.View.Deployments[0].Pods) != 0 {
		t.Fatalf("top-level Pods were projected without verified ownership: %#v", result.View.Deployments[0].Pods)
	}
	if result.Metrics.SourceObjects != 5 || result.Metrics.ProjectedObjects != 4 || result.Metrics.TopLevelEntities != 4 {
		t.Fatalf("object metrics = %#v", result.Metrics)
	}
	if result.Metrics.UnprojectedPods != 1 || result.Metrics.OmittedContainers != 1 || result.Metrics.OmittedServicePorts != 1 || result.Metrics.OmittedNamedTargetPorts != 0 {
		t.Fatalf("omission metrics = %#v", result.Metrics)
	}
	if result.Metrics.SourceBytes != len(result.SourceJSON) || result.Metrics.DocumentJSONBytes != len(result.DocumentJSON) || result.Metrics.ANFBytes != len(result.ANF) {
		t.Fatalf("byte metrics = %#v", result.Metrics)
	}
	wantReduction := float64(len(result.DocumentJSON)-len(result.ANF)) / float64(len(result.DocumentJSON)) * 100
	if result.Metrics.Reduction != wantReduction {
		t.Fatalf("reduction = %.8f, want document reduction %.8f", result.Metrics.Reduction, wantReduction)
	}
	if result.Metrics.DocumentJSONTokensEst != estimateTokens(len(result.DocumentJSON)) || result.Metrics.ANFTokensEst != estimateTokens(len(result.ANF)) {
		t.Fatalf("token estimates = %#v", result.Metrics)
	}

	var decoded anf.Document
	if err := json.Unmarshal(result.DocumentJSON, &decoded); err != nil {
		t.Fatalf("unmarshal document JSON: %v", err)
	}
	assertDocumentFactsEqual(t, result.Document, &decoded)
	if encoded := anf.EncodeToString(&decoded); encoded != result.ANF {
		t.Fatalf("document JSON re-encodes to different ANF:\n%s\n%s", encoded, result.ANF)
	}
	if strings.Contains(string(result.DocumentJSON), "CPUPercent") || strings.Contains(string(result.DocumentJSON), "Events") {
		t.Fatalf("document JSON includes source fields outside the ANF document: %s", result.DocumentJSON)
	}
}

func TestCaptureCountsUnprojectedPodsWithoutChangingDocument(t *testing.T) {
	baselineSource := fixtureSource()
	changedSource := fixtureSource()
	extraPod := *changedSource.Pods.Items[0].DeepCopy()
	extraPod.Name = "operator-abc123-x2"
	extraPod.UID = types.UID("pod-2")
	changedSource.Pods.Items = append(changedSource.Pods.Items, extraPod)

	baseline := mustCapture(t, baselineSource)
	changed := mustCapture(t, changedSource)

	if string(baseline.SourceJSON) == string(changed.SourceJSON) || baseline.Metrics.SourceBytes == changed.Metrics.SourceBytes {
		t.Fatal("source payload did not reflect changed Pod data")
	}
	if baseline.Metrics.UnprojectedPods != 1 || changed.Metrics.UnprojectedPods != 2 {
		t.Fatalf("unprojected Pods = %d -> %d, want 1 -> 2", baseline.Metrics.UnprojectedPods, changed.Metrics.UnprojectedPods)
	}
	assertComparisonUnchanged(t, baseline, changed)
}

func TestCaptureCountsOmittedNamedTargetPortWithoutChangingDocument(t *testing.T) {
	baselineSource := fixtureSource()
	changedSource := fixtureSource()
	baselineSource.Services.Items[0].Spec.Ports[0].TargetPort = intstr.FromInt32(0)
	changedSource.Services.Items[0].Spec.Ports[0].TargetPort = intstr.FromString("metrics")
	if changedSource.Services.Items[0].Spec.Ports[0].TargetPort.Type != intstr.String {
		t.Fatalf("fixture targetPort type = %d, want String", changedSource.Services.Items[0].Spec.Ports[0].TargetPort.Type)
	}
	if projected := translateService(changedSource.Services.Items[0]); projected.Port != 0 || projected.TargetPort != 0 {
		t.Fatalf("direct named targetPort pair was projected: %#v", projected)
	}

	baseline := mustCapture(t, baselineSource)
	changed := mustCapture(t, changedSource)

	if string(baseline.SourceJSON) == string(changed.SourceJSON) || baseline.Metrics.SourceBytes == changed.Metrics.SourceBytes {
		t.Fatal("source payload did not reflect named targetPort")
	}
	if baseline.Metrics.OmittedNamedTargetPorts != 0 || changed.Metrics.OmittedNamedTargetPorts != 1 {
		t.Fatalf("omitted named target ports = %d -> %d, want 0 -> 1", baseline.Metrics.OmittedNamedTargetPorts, changed.Metrics.OmittedNamedTargetPorts)
	}
	assertComparisonUnchanged(t, baseline, changed)
	if changed.View.Services[0].Port != 0 || changed.View.Services[0].TargetPort != 0 {
		t.Fatalf("named targetPort pair was projected: %#v", changed.View.Services[0])
	}
}

func TestCaptureCountsAdditionalContainersAndServicePortsWithoutChangingDocument(t *testing.T) {
	baselineSource := fixtureSource()
	changedSource := fixtureSource()
	changedSource.Deployments.Items[0].Spec.Template.Spec.Containers = append(
		changedSource.Deployments.Items[0].Spec.Template.Spec.Containers,
		corev1.Container{Name: "telemetry", Image: "ghcr.io/clawdlinux/telemetry:v1"},
	)
	changedSource.Services.Items[0].Spec.Ports = append(
		changedSource.Services.Items[0].Spec.Ports,
		corev1.ServicePort{Name: "admin", Port: 9090, TargetPort: intstr.FromInt32(9090)},
	)

	baseline := mustCapture(t, baselineSource)
	changed := mustCapture(t, changedSource)

	if string(baseline.SourceJSON) == string(changed.SourceJSON) || baseline.Metrics.SourceBytes == changed.Metrics.SourceBytes {
		t.Fatal("source payload did not reflect additional container and port changes")
	}
	if baseline.Metrics.OmittedContainers != 1 || changed.Metrics.OmittedContainers != 2 {
		t.Fatalf("omitted containers = %d -> %d, want 1 -> 2", baseline.Metrics.OmittedContainers, changed.Metrics.OmittedContainers)
	}
	if baseline.Metrics.OmittedServicePorts != 1 || changed.Metrics.OmittedServicePorts != 2 {
		t.Fatalf("omitted service ports = %d -> %d, want 1 -> 2", baseline.Metrics.OmittedServicePorts, changed.Metrics.OmittedServicePorts)
	}
	assertComparisonUnchanged(t, baseline, changed)
}

func TestCaptureStableSortsAllFiveLists(t *testing.T) {
	ordered := twoItemSource()
	permuted := reverseItems(twoItemSource())

	first := mustCapture(t, ordered)
	second := mustCapture(t, permuted)

	if string(first.SourceJSON) != string(second.SourceJSON) {
		t.Fatalf("source JSON changed with list order:\n%s\n%s", first.SourceJSON, second.SourceJSON)
	}
	if string(first.DocumentJSON) != string(second.DocumentJSON) || first.ANF != second.ANF || !reflect.DeepEqual(first.Metrics, second.Metrics) {
		t.Fatal("projection, ANF, or metrics changed with list order")
	}
}

func TestCaptureFailedJobConditionTakesPrecedence(t *testing.T) {
	source := fixtureSource()
	source.Jobs.Items[0].Status.Succeeded = 1
	source.Jobs.Items[0].Status.Conditions = []batchv1.JobCondition{
		{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
		{Type: batchv1.JobFailed, Status: corev1.ConditionTrue},
	}

	result := mustCapture(t, source)
	job := result.View.Jobs[0]
	if !job.Completed || job.Succeeded {
		t.Fatalf("failed partial Job = %#v, want completed=true succeeded=false", job)
	}
	if !strings.Contains(result.ANF, "job snapshot-job [failing]") {
		t.Fatalf("failed Job status missing from ANF: %q", result.ANF)
	}
}

func TestCaptureFailsClosedWhenAnyRequiredListFails(t *testing.T) {
	for _, resource := range []string{"deployments", "pods", "services", "jobs", "cronjobs"} {
		t.Run(resource, func(t *testing.T) {
			lister := newStaticLister(fixtureSource())
			lister.failResource = resource
			result, err := Capture(context.Background(), lister, testOptions())
			if err == nil || !strings.Contains(err.Error(), resource) {
				t.Fatalf("error = %v, want %s list error", err, resource)
			}
			if result != nil {
				t.Fatalf("result = %#v, want nil", result)
			}
		})
	}
}

func TestCaptureHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := Capture(ctx, blockingLister{}, testOptions())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if result != nil {
		t.Fatalf("result = %#v, want nil", result)
	}
}

func TestCaptureHonorsDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	result, err := Capture(ctx, blockingLister{}, testOptions())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context.DeadlineExceeded", err)
	}
	if result != nil {
		t.Fatalf("result = %#v, want nil", result)
	}
}

func TestCaptureRejectsUnsafeSummaryLabels(t *testing.T) {
	for _, options := range []Options{
		{Cluster: "bad cluster", Namespace: "agentic-system", Clock: func() time.Time { return fixtureNow }},
		{Cluster: "bad\tcluster", Namespace: "agentic-system", Clock: func() time.Time { return fixtureNow }},
		{Cluster: "bad\ncluster", Namespace: "agentic-system", Clock: func() time.Time { return fixtureNow }},
		{Cluster: "bad/cluster", Namespace: "agentic-system", Clock: func() time.Time { return fixtureNow }},
		{Cluster: "-bad", Namespace: "agentic-system", Clock: func() time.Time { return fixtureNow }},
		{Cluster: "bad-", Namespace: "agentic-system", Clock: func() time.Time { return fixtureNow }},
		{Cluster: strings.Repeat("a", 129), Namespace: "agentic-system", Clock: func() time.Time { return fixtureNow }},
		{Cluster: "showcase-cluster", Namespace: "bad namespace", Clock: func() time.Time { return fixtureNow }},
		{Cluster: "showcase-cluster", Namespace: "bad\tnamespace", Clock: func() time.Time { return fixtureNow }},
		{Cluster: "showcase-cluster", Namespace: "bad\rnamespace", Clock: func() time.Time { return fixtureNow }},
		{Cluster: "showcase-cluster", Namespace: "bad\x00namespace", Clock: func() time.Time { return fixtureNow }},
		{Cluster: "showcase-cluster", Namespace: "Bad", Clock: func() time.Time { return fixtureNow }},
		{Cluster: "showcase-cluster", Namespace: "bad_namespace", Clock: func() time.Time { return fixtureNow }},
		{Cluster: "showcase-cluster", Namespace: "-bad", Clock: func() time.Time { return fixtureNow }},
		{Cluster: "showcase-cluster", Namespace: "bad-", Clock: func() time.Time { return fixtureNow }},
		{Cluster: "showcase-cluster", Namespace: strings.Repeat("a", 64), Clock: func() time.Time { return fixtureNow }},
	} {
		if _, err := Capture(context.Background(), newStaticLister(fixtureSource()), options); err == nil {
			t.Fatalf("Capture accepted unsafe summary label in %#v", options)
		}
	}
}

func TestCaptureAcceptsVisualizerCompatibleSummaryLabels(t *testing.T) {
	options := testOptions()
	options.Cluster = "Kind.Demo_1"
	options.Namespace = "agentic-system"

	if _, err := Capture(context.Background(), newStaticLister(fixtureSource()), options); err != nil {
		t.Fatalf("Capture rejected visualizer-compatible labels: %v", err)
	}
}

func TestSummaryAndPreviewAreBounded(t *testing.T) {
	result := mustCapture(t, fixtureSource())
	pattern := regexp.MustCompile(`^ANF context: source=kubernetes/showcase-cluster scope=namespace:agentic-system source_bytes=[1-9][0-9]* source_objects=5 projected_objects=4 unprojected_pods=1 omitted_containers=1 omitted_service_ports=1 omitted_named_target_ports=0 document_json_bytes=[1-9][0-9]* anf_bytes=[1-9][0-9]* document_json_tokens_est=[1-9][0-9]* anf_tokens_est=[1-9][0-9]* reduction=-?[0-9]+\.[0-9] top_level_entities=4$`)
	if !pattern.MatchString(result.Summary()) {
		t.Fatalf("summary is not parseable: %q", result.Summary())
	}
	if lines := result.PreviewLines(3); len(lines) == 0 || len(lines) > 3 {
		t.Fatalf("preview has %d lines, want 1 to 3", len(lines))
	}
}

func TestWriteArtifactAtomicallyReplacesSymlinkWithPrivateFile(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target.anf")
	destination := filepath.Join(directory, "snapshot.anf")
	if err := os.WriteFile(target, []byte("target-content"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.Symlink(target, destination); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	if err := WriteArtifact(destination, "snapshot-content"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}
	info, err := os.Lstat(destination)
	if err != nil {
		t.Fatalf("lstat destination: %v", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("destination mode = %v, want regular 0600", info.Mode())
	}
	targetContent, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(targetContent) != "target-content" {
		t.Fatalf("symlink target changed to %q", targetContent)
	}
}

func TestWriteArtifactRequiresParentAndCleansTempOnRenameFailure(t *testing.T) {
	directory := t.TempDir()
	missing := filepath.Join(directory, "missing", "snapshot.anf")
	if err := WriteArtifact(missing, "data"); err == nil {
		t.Fatal("expected missing parent error")
	}
	if _, err := os.Stat(filepath.Dir(missing)); !os.IsNotExist(err) {
		t.Fatalf("missing parent was created or stat failed: %v", err)
	}

	destination := filepath.Join(directory, "existing-directory")
	if err := os.Mkdir(destination, 0o755); err != nil {
		t.Fatalf("create destination directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(destination, "child"), []byte("x"), 0o600); err != nil {
		t.Fatalf("make destination nonempty: %v", err)
	}
	if err := WriteArtifact(destination, "data"); err == nil {
		t.Fatal("expected rename over nonempty directory to fail")
	}
	temps, err := filepath.Glob(filepath.Join(directory, ".existing-directory.tmp-*"))
	if err != nil {
		t.Fatalf("glob temporary files: %v", err)
	}
	if len(temps) != 0 {
		t.Fatalf("temporary files left after failure: %v", temps)
	}
}

func assertComparisonUnchanged(t *testing.T, baseline, changed *Result) {
	t.Helper()
	if string(baseline.DocumentJSON) != string(changed.DocumentJSON) {
		t.Fatalf("document JSON changed:\n%s\n%s", baseline.DocumentJSON, changed.DocumentJSON)
	}
	if baseline.ANF != changed.ANF {
		t.Fatalf("ANF changed:\n%s\n%s", baseline.ANF, changed.ANF)
	}
	if baseline.Metrics.DocumentJSONBytes != changed.Metrics.DocumentJSONBytes || baseline.Metrics.ANFBytes != changed.Metrics.ANFBytes ||
		baseline.Metrics.DocumentJSONTokensEst != changed.Metrics.DocumentJSONTokensEst || baseline.Metrics.ANFTokensEst != changed.Metrics.ANFTokensEst ||
		baseline.Metrics.Reduction != changed.Metrics.Reduction {
		t.Fatalf("comparison metrics changed: %#v != %#v", baseline.Metrics, changed.Metrics)
	}
}

func assertDocumentFactsEqual(t *testing.T, want, got *anf.Document) {
	t.Helper()
	if !reflect.DeepEqual(want.Headers, got.Headers) {
		t.Fatalf("headers differ: %#v != %#v", want.Headers, got.Headers)
	}
	if !reflect.DeepEqual(want.Entities, got.Entities) {
		t.Fatalf("entities, statuses, or properties differ: %#v != %#v", want.Entities, got.Entities)
	}
	if !reflect.DeepEqual(want.Alerts, got.Alerts) {
		t.Fatalf("alerts differ: %#v != %#v", want.Alerts, got.Alerts)
	}
	if !reflect.DeepEqual(want.Actions, got.Actions) {
		t.Fatalf("actions differ: %#v != %#v", want.Actions, got.Actions)
	}
}

func mustCapture(t *testing.T, source SourceSnapshot) *Result {
	t.Helper()
	result, err := Capture(context.Background(), newStaticLister(source), testOptions())
	if err != nil {
		t.Fatalf("Capture returned error: %v", err)
	}
	return result
}

func testOptions() Options {
	return Options{Cluster: "showcase-cluster", Namespace: "agentic-system", Clock: func() time.Time { return fixtureNow }}
}

type staticLister struct {
	source       SourceSnapshot
	calls        []string
	failResource string
}

func newStaticLister(source SourceSnapshot) *staticLister {
	return &staticLister{source: source}
}

func (lister *staticLister) result(resource string, object any) error {
	lister.calls = append(lister.calls, resource)
	if lister.failResource == resource {
		return errors.New("denied")
	}
	return nil
}

func (lister *staticLister) ListDeployments(context.Context, string) (*appsv1.DeploymentList, error) {
	if err := lister.result("deployments", &lister.source.Deployments); err != nil {
		return nil, err
	}
	return lister.source.Deployments.DeepCopy(), nil
}

func (lister *staticLister) ListPods(context.Context, string) (*corev1.PodList, error) {
	if err := lister.result("pods", &lister.source.Pods); err != nil {
		return nil, err
	}
	return lister.source.Pods.DeepCopy(), nil
}

func (lister *staticLister) ListServices(context.Context, string) (*corev1.ServiceList, error) {
	if err := lister.result("services", &lister.source.Services); err != nil {
		return nil, err
	}
	return lister.source.Services.DeepCopy(), nil
}

func (lister *staticLister) ListJobs(context.Context, string) (*batchv1.JobList, error) {
	if err := lister.result("jobs", &lister.source.Jobs); err != nil {
		return nil, err
	}
	return lister.source.Jobs.DeepCopy(), nil
}

func (lister *staticLister) ListCronJobs(context.Context, string) (*batchv1.CronJobList, error) {
	if err := lister.result("cronjobs", &lister.source.CronJobs); err != nil {
		return nil, err
	}
	return lister.source.CronJobs.DeepCopy(), nil
}

type blockingLister struct{}

func (blockingLister) ListDeployments(ctx context.Context, _ string) (*appsv1.DeploymentList, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (blockingLister) ListPods(context.Context, string) (*corev1.PodList, error) {
	panic("ListPods called after cancellation")
}

func (blockingLister) ListServices(context.Context, string) (*corev1.ServiceList, error) {
	panic("ListServices called after cancellation")
}

func (blockingLister) ListJobs(context.Context, string) (*batchv1.JobList, error) {
	panic("ListJobs called after cancellation")
}

func (blockingLister) ListCronJobs(context.Context, string) (*batchv1.CronJobList, error) {
	panic("ListCronJobs called after cancellation")
}

func fixtureSource() SourceSnapshot {
	replicas := int32(3)
	start := metav1.NewTime(fixtureNow.Add(-10 * time.Minute))
	completion := metav1.NewTime(fixtureNow.Add(-5 * time.Minute))
	lastSchedule := metav1.NewTime(fixtureNow.Add(-time.Hour))
	annotations := map[string]string{"example.com/build": strings.Repeat("build-metadata-", 20)}

	return SourceSnapshot{
		Deployments: appsv1.DeploymentList{Items: []appsv1.Deployment{{
			ObjectMeta: metav1.ObjectMeta{Name: "operator", Namespace: "agentic-system", UID: types.UID("deployment-1"), CreationTimestamp: metav1.NewTime(fixtureNow.Add(-49 * time.Hour)), Annotations: annotations},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{
					{Name: "manager", Image: "ghcr.io/clawdlinux/operator:v1.2.3"},
					{Name: "sidecar", Image: "ghcr.io/clawdlinux/sidecar:v1"},
				}}},
				Strategy: appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType},
			},
			Status: appsv1.DeploymentStatus{ReadyReplicas: 2},
		}}},
		Pods: corev1.PodList{Items: []corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{Name: "operator-abc123-x1", Namespace: "agentic-system", UID: types.UID("pod-1"), Annotations: annotations},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "manager", Image: "ghcr.io/clawdlinux/operator:v1.2.3"}}},
		}}},
		Services: corev1.ServiceList{Items: []corev1.Service{{
			ObjectMeta: metav1.ObjectMeta{Name: "operator", Namespace: "agentic-system", UID: types.UID("service-1"), Annotations: annotations},
			Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, Ports: []corev1.ServicePort{
				{Name: "https", Port: 8443, TargetPort: intstr.FromInt32(9443)},
				{Name: "metrics", Port: 8080, TargetPort: intstr.FromString("metrics")},
			}},
		}}},
		Jobs: batchv1.JobList{Items: []batchv1.Job{{
			ObjectMeta: metav1.ObjectMeta{Name: "snapshot-job", Namespace: "agentic-system", UID: types.UID("job-1"), Annotations: annotations},
			Status: batchv1.JobStatus{
				StartTime: &start, CompletionTime: &completion, Succeeded: 1,
				Conditions: []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}},
			},
		}}},
		CronJobs: batchv1.CronJobList{Items: []batchv1.CronJob{{
			ObjectMeta: metav1.ObjectMeta{Name: "snapshot-cron", Namespace: "agentic-system", UID: types.UID("cronjob-1"), Annotations: annotations},
			Spec:       batchv1.CronJobSpec{Schedule: "*/5 * * * *"},
			Status:     batchv1.CronJobStatus{LastScheduleTime: &lastSchedule},
		}}},
	}
}

func twoItemSource() SourceSnapshot {
	source := fixtureSource()
	second := fixtureSource()
	second.Deployments.Items[0].Name, second.Deployments.Items[0].UID = "api", types.UID("deployment-2")
	second.Pods.Items[0].Name, second.Pods.Items[0].UID = "api-pod", types.UID("pod-2")
	second.Services.Items[0].Name, second.Services.Items[0].UID = "api", types.UID("service-2")
	second.Jobs.Items[0].Name, second.Jobs.Items[0].UID = "api-job", types.UID("job-2")
	second.CronJobs.Items[0].Name, second.CronJobs.Items[0].UID = "api-cron", types.UID("cronjob-2")
	source.Deployments.Items = append(second.Deployments.Items, source.Deployments.Items...)
	source.Pods.Items = append(second.Pods.Items, source.Pods.Items...)
	source.Services.Items = append(second.Services.Items, source.Services.Items...)
	source.Jobs.Items = append(second.Jobs.Items, source.Jobs.Items...)
	source.CronJobs.Items = append(second.CronJobs.Items, source.CronJobs.Items...)
	return source
}

func reverseItems(source SourceSnapshot) SourceSnapshot {
	reverse := func(length int, swap func(int, int)) {
		for left, right := 0, length-1; left < right; left, right = left+1, right-1 {
			swap(left, right)
		}
	}
	reverse(len(source.Deployments.Items), func(left, right int) {
		source.Deployments.Items[left], source.Deployments.Items[right] = source.Deployments.Items[right], source.Deployments.Items[left]
	})
	reverse(len(source.Pods.Items), func(left, right int) {
		source.Pods.Items[left], source.Pods.Items[right] = source.Pods.Items[right], source.Pods.Items[left]
	})
	reverse(len(source.Services.Items), func(left, right int) {
		source.Services.Items[left], source.Services.Items[right] = source.Services.Items[right], source.Services.Items[left]
	})
	reverse(len(source.Jobs.Items), func(left, right int) {
		source.Jobs.Items[left], source.Jobs.Items[right] = source.Jobs.Items[right], source.Jobs.Items[left]
	})
	reverse(len(source.CronJobs.Items), func(left, right int) {
		source.CronJobs.Items[left], source.CronJobs.Items[right] = source.CronJobs.Items[right], source.CronJobs.Items[left]
	})
	return source
}
