package anfsnapshot

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var fixtureNow = time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)

func TestCaptureConvertsObservedNamespaceDeterministically(t *testing.T) {
	client := fixtureClient()
	options := Options{Cluster: "kind-showcase", Namespace: "agentic-system", Now: fixtureNow}

	first, err := Capture(context.Background(), NewKubernetesLister(client), options)
	if err != nil {
		t.Fatalf("Capture returned error: %v", err)
	}
	second, err := Capture(context.Background(), NewKubernetesLister(client), options)
	if err != nil {
		t.Fatalf("second Capture returned error: %v", err)
	}

	if string(first.RawJSON) != string(second.RawJSON) {
		t.Fatal("raw snapshot changed across identical captures")
	}
	if first.ANF != second.ANF {
		t.Fatal("ANF changed across identical captures")
	}
	if len(first.View.Deployments) != 1 || len(first.View.Services) != 1 || len(first.View.Jobs) != 1 || len(first.View.CronJobs) != 1 {
		t.Fatalf("unexpected view entity counts: %#v", first.View)
	}

	deployment := first.View.Deployments[0]
	if deployment.Name != "operator" || deployment.Replicas != 3 || deployment.ReadyReplicas != 2 {
		t.Fatalf("deployment identity or replicas not observed: %#v", deployment)
	}
	if deployment.Image != "ghcr.io/clawdlinux/operator:v1.2.3" || deployment.Strategy != "RollingUpdate" || deployment.AgeDays != 2 {
		t.Fatalf("deployment fields not converted from observed state: %#v", deployment)
	}
	if len(deployment.Pods) != 0 {
		t.Fatalf("deployment pods = %d, want none without verified ownership", len(deployment.Pods))
	}

	service := first.View.Services[0]
	if service.Name != "operator" || service.Type != "ClusterIP" || service.Port != 8443 || service.TargetPort != 9443 {
		t.Fatalf("service fields not converted from observed state: %#v", service)
	}
	if service.Endpoints != 0 || service.TotalPods != 0 {
		t.Fatalf("service endpoint data was invented: %#v", service)
	}

	job := first.View.Jobs[0]
	if !job.Completed || !job.Succeeded || !job.LastRun.Equal(fixtureNow.Add(-10*time.Minute)) || job.Duration != 5*time.Minute {
		t.Fatalf("job status or times not converted: %#v", job)
	}
	cronJob := first.View.CronJobs[0]
	if cronJob.Schedule != "*/5 * * * *" || !cronJob.LastRun.Equal(fixtureNow.Add(-time.Hour)) || !cronJob.NextRun.IsZero() {
		t.Fatalf("cronjob fields not converted conservatively: %#v", cronJob)
	}

	if len(first.Raw.Pods.Items) != 1 {
		t.Fatalf("raw pods = %d, want 1", len(first.Raw.Pods.Items))
	}
	rawJSON, err := json.Marshal(first.Raw)
	if err != nil {
		t.Fatalf("marshal raw snapshot: %v", err)
	}
	if string(rawJSON) != string(first.RawJSON) {
		t.Fatal("RawJSON is not the exact deterministic five-list struct encoding")
	}
}

func TestCaptureFailsClosedWhenAnyRequiredListFails(t *testing.T) {
	tests := []struct {
		name     string
		resource string
	}{
		{name: "deployments", resource: "deployments"},
		{name: "pods", resource: "pods"},
		{name: "services", resource: "services"},
		{name: "jobs", resource: "jobs"},
		{name: "cronjobs", resource: "cronjobs"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := fixtureClient()
			client.PrependReactor("list", test.resource, func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: test.resource}, "", errors.New("denied"))
			})

			result, err := Capture(context.Background(), NewKubernetesLister(client), Options{
				Cluster: "kind-showcase", Namespace: "agentic-system", Now: fixtureNow,
			})
			if err == nil {
				t.Fatalf("expected %s list error", test.resource)
			}
			if result != nil {
				t.Fatalf("result = %#v, want nil on partial snapshot", result)
			}
			if !strings.Contains(err.Error(), test.resource) {
				t.Fatalf("error %q does not identify failed source %q", err, test.resource)
			}
		})
	}
}

func TestCaptureDoesNotInventMetricsActionsOrEvents(t *testing.T) {
	result, err := Capture(context.Background(), NewKubernetesLister(fixtureClient()), Options{
		Cluster: "kind-showcase", Namespace: "agentic-system", Now: fixtureNow,
	})
	if err != nil {
		t.Fatalf("Capture returned error: %v", err)
	}

	deployment := result.View.Deployments[0]
	if deployment.CPUPercent != 0 || deployment.MemPercent != 0 || deployment.RequestsPerSec != 0 || deployment.ErrorRate != 0 {
		t.Fatalf("unobserved deployment metrics were populated: %#v", deployment)
	}
	if len(result.View.Events) != 0 {
		t.Fatalf("events = %d, want 0", len(result.View.Events))
	}
	if result.View.AgentPermissions.CanScale || result.View.AgentPermissions.CanRollout ||
		result.View.AgentPermissions.CanRestart || result.View.AgentPermissions.CanLogs ||
		result.View.AgentPermissions.CanExec || result.View.AgentPermissions.CanDescribe {
		t.Fatalf("unverified permissions were enabled: %#v", result.View.AgentPermissions)
	}
	if len(result.Document.Actions) != 0 || strings.Contains(result.ANF, "\n?") {
		t.Fatalf("ANF contains unverified actions: %q", result.ANF)
	}
	if strings.Contains(result.ANF, "cpu:") || strings.Contains(result.ANF, "mem:") || strings.Contains(result.ANF, "requests:") || strings.Contains(result.ANF, "errors:") {
		t.Fatalf("ANF contains unobserved metrics: %q", result.ANF)
	}
}

func TestCaptureMeasuresExactOutputAndReportsReduction(t *testing.T) {
	result, err := Capture(context.Background(), NewKubernetesLister(fixtureClient()), Options{
		Cluster: "kind-showcase", Namespace: "agentic-system", Now: fixtureNow,
	})
	if err != nil {
		t.Fatalf("Capture returned error: %v", err)
	}

	if result.Metrics.RawBytes != len(result.ProjectedJSON) || result.Metrics.ANFBytes != len(result.ANF) {
		t.Fatalf("byte measurements = %#v, projected=%d anf=%d", result.Metrics, len(result.ProjectedJSON), len(result.ANF))
	}
	if result.Metrics.RawTokensEst != estimateTokens(len(result.ProjectedJSON)) || result.Metrics.ANFTokensEst != estimateTokens(len(result.ANF)) {
		t.Fatalf("token estimates do not use chars/4: %#v", result.Metrics)
	}
	wantReduction := float64(len(result.ProjectedJSON)-len(result.ANF)) / float64(len(result.ProjectedJSON)) * 100
	if result.Metrics.Reduction != wantReduction {
		t.Fatalf("reduction = %.8f, want byte reduction %.8f", result.Metrics.Reduction, wantReduction)
	}
	if result.Metrics.Reduction <= 0 {
		t.Fatalf("reduction = %.1f, want positive for realistic fixture", result.Metrics.Reduction)
	}
	if result.Metrics.Entities != len(result.Document.Entities) || result.Metrics.Entities != 4 {
		t.Fatalf("entities = %d, document has %d", result.Metrics.Entities, len(result.Document.Entities))
	}

	summaryPattern := regexp.MustCompile(`^ANF context: source=kubernetes/kind-showcase scope=namespace:agentic-system raw_bytes=[1-9][0-9]* anf_bytes=[1-9][0-9]* raw_tokens_est=[1-9][0-9]* anf_tokens_est=[1-9][0-9]* reduction=-?[0-9]+\.[0-9] entities=4$`)
	if !summaryPattern.MatchString(result.Summary()) {
		t.Fatalf("summary is not parseable: %q", result.Summary())
	}
}

func TestCaptureExcludesUnprojectedPodDataFromReduction(t *testing.T) {
	baseline, err := Capture(context.Background(), NewKubernetesLister(fixtureClient()), Options{
		Cluster: "kind-showcase", Namespace: "agentic-system", Now: fixtureNow,
	})
	if err != nil {
		t.Fatalf("baseline Capture returned error: %v", err)
	}

	client := fixtureClient()
	pod, err := client.CoreV1().Pods("agentic-system").Get(context.Background(), "operator-abc123-x1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get fixture pod: %v", err)
	}
	pod.Annotations["example.com/build"] = strings.Repeat("different-unprojected-metadata", 40)
	pod.Spec.Containers[0].Image = "ghcr.io/clawdlinux/operator:unprojected"
	if _, err := client.CoreV1().Pods("agentic-system").Update(context.Background(), pod, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update fixture pod: %v", err)
	}

	changed, err := Capture(context.Background(), NewKubernetesLister(client), Options{
		Cluster: "kind-showcase", Namespace: "agentic-system", Now: fixtureNow,
	})
	if err != nil {
		t.Fatalf("changed Capture returned error: %v", err)
	}

	if string(baseline.RawJSON) == string(changed.RawJSON) {
		t.Fatal("source JSON did not reflect changed Pod data")
	}
	if baseline.ANF != changed.ANF {
		t.Fatal("unprojected Pod data changed ANF")
	}
	if baseline.Metrics.Reduction != changed.Metrics.Reduction {
		t.Fatalf("unprojected Pod data changed reduction: %.8f != %.8f", baseline.Metrics.Reduction, changed.Metrics.Reduction)
	}
}

func TestWriteArtifactUsesPrivateModeAndDoesNotCreateParents(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "snapshot.anf")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatalf("seed existing artifact: %v", err)
	}

	if err := WriteArtifact(path, "@source kubernetes/test\n"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat artifact: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("artifact mode = %04o, want 0600", info.Mode().Perm())
	}

	missingParent := filepath.Join(directory, "missing", "snapshot.anf")
	if err := WriteArtifact(missingParent, "data"); err == nil {
		t.Fatal("expected error for missing output parent")
	}
	if _, err := os.Stat(filepath.Dir(missingParent)); !os.IsNotExist(err) {
		t.Fatalf("missing parent was created or returned unexpected error: %v", err)
	}
}

func TestCaptureHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := Capture(ctx, blockingLister{}, Options{
		Cluster: "kind-showcase", Namespace: "agentic-system", Now: fixtureNow,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if result != nil {
		t.Fatalf("result = %#v, want nil", result)
	}
}

func TestCaptureHonorsContextDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	result, err := Capture(ctx, blockingLister{}, Options{
		Cluster: "kind-showcase", Namespace: "agentic-system", Now: fixtureNow,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context.DeadlineExceeded", err)
	}
	if result != nil {
		t.Fatalf("result = %#v, want nil", result)
	}
}

type blockingLister struct{}

func (blockingLister) ListDeployments(ctx context.Context, _ string) (*appsv1.DeploymentList, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (blockingLister) ListPods(context.Context, string) (*corev1.PodList, error) {
	panic("ListPods called after deployment cancellation")
}

func (blockingLister) ListServices(context.Context, string) (*corev1.ServiceList, error) {
	panic("ListServices called after deployment cancellation")
}

func (blockingLister) ListJobs(context.Context, string) (*batchv1.JobList, error) {
	panic("ListJobs called after deployment cancellation")
}

func (blockingLister) ListCronJobs(context.Context, string) (*batchv1.CronJobList, error) {
	panic("ListCronJobs called after deployment cancellation")
}

func fixtureClient() *fake.Clientset {
	replicas := int32(3)
	start := metav1.NewTime(fixtureNow.Add(-10 * time.Minute))
	completion := metav1.NewTime(fixtureNow.Add(-5 * time.Minute))
	lastSchedule := metav1.NewTime(fixtureNow.Add(-time.Hour))
	commonAnnotations := map[string]string{
		"example.com/build":  strings.Repeat("build-metadata-", 20),
		"example.com/owner":  "showcase-team",
		"example.com/source": "live-cluster-fixture",
	}

	return fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "operator", Namespace: "agentic-system", CreationTimestamp: metav1.NewTime(fixtureNow.Add(-49 * time.Hour)), Annotations: commonAnnotations,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "operator"}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "operator", "pod-template-hash": "abc123"}},
					Spec: corev1.PodSpec{Containers: []corev1.Container{
						{Name: "manager", Image: "ghcr.io/clawdlinux/operator:v1.2.3"},
						{Name: "sidecar", Image: "ghcr.io/clawdlinux/sidecar:v1"},
					}},
				},
				Strategy: appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType},
			},
			Status: appsv1.DeploymentStatus{ReadyReplicas: 2},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "operator-abc123-x1", Namespace: "agentic-system", Labels: map[string]string{"app": "operator", "pod-template-hash": "abc123"}, Annotations: commonAnnotations},
			Spec:       corev1.PodSpec{NodeName: "kind-worker", Containers: []corev1.Container{{Name: "manager", Image: "ghcr.io/clawdlinux/operator:v1.2.3"}}},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "operator", Namespace: "agentic-system", Annotations: commonAnnotations},
			Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, Ports: []corev1.ServicePort{{
				Name: "https", Port: 8443, TargetPort: intstr.FromInt32(9443),
			}}},
		},
		&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{Name: "snapshot-job", Namespace: "agentic-system", Annotations: commonAnnotations},
			Status: batchv1.JobStatus{
				StartTime: &start, CompletionTime: &completion, Succeeded: 1,
				Conditions: []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}},
			},
		},
		&batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{Name: "snapshot-cron", Namespace: "agentic-system", Annotations: commonAnnotations},
			Spec:       batchv1.CronJobSpec{Schedule: "*/5 * * * *"},
			Status:     batchv1.CronJobStatus{LastScheduleTime: &lastSchedule},
		},
	)
}
