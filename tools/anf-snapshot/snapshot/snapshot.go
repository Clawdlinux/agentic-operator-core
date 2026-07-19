package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Clawdlinux/agent-native-format/pkg/anf"
	anfkubernetes "github.com/Clawdlinux/agent-native-format/translators/kubernetes"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

var (
	summaryClusterPattern   = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9._-]{0,126}[A-Za-z0-9])?$`)
	summaryNamespacePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
)

// Lister provides the five sequential Kubernetes reads used by a snapshot.
// The result is best-effort across calls rather than a transactional cluster view.
type Lister interface {
	ListDeployments(context.Context, string) (*appsv1.DeploymentList, error)
	ListPods(context.Context, string) (*corev1.PodList, error)
	ListServices(context.Context, string) (*corev1.ServiceList, error)
	ListJobs(context.Context, string) (*batchv1.JobList, error)
	ListCronJobs(context.Context, string) (*batchv1.CronJobList, error)
}

type kubernetesLister struct {
	client kubernetes.Interface
}

// NewKubernetesLister adapts a client-go clientset to Lister.
func NewKubernetesLister(client kubernetes.Interface) Lister {
	return kubernetesLister{client: client}
}

func (lister kubernetesLister) ListDeployments(ctx context.Context, namespace string) (*appsv1.DeploymentList, error) {
	return lister.client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
}

func (lister kubernetesLister) ListPods(ctx context.Context, namespace string) (*corev1.PodList, error) {
	return lister.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
}

func (lister kubernetesLister) ListServices(ctx context.Context, namespace string) (*corev1.ServiceList, error) {
	return lister.client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
}

func (lister kubernetesLister) ListJobs(ctx context.Context, namespace string) (*batchv1.JobList, error) {
	return lister.client.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
}

func (lister kubernetesLister) ListCronJobs(ctx context.Context, namespace string) (*batchv1.CronJobList, error) {
	return lister.client.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{})
}

// Options identifies the source and supplies a clock read after all lists finish.
type Options struct {
	Cluster   string
	Namespace string
	Clock     func() time.Time
}

// SourceSnapshot contains the exact five fetched list payloads after stable ordering.
type SourceSnapshot struct {
	Deployments appsv1.DeploymentList `json:"deployments"`
	Pods        corev1.PodList        `json:"pods"`
	Services    corev1.ServiceList    `json:"services"`
	Jobs        batchv1.JobList       `json:"jobs"`
	CronJobs    batchv1.CronJobList   `json:"cronJobs"`
}

// Metrics separates source payload accounting from the exact document encoding comparison.
type Metrics struct {
	SourceBytes             int
	SourceObjects           int
	ProjectedObjects        int
	UnprojectedPods         int
	OmittedContainers       int
	OmittedServicePorts     int
	OmittedNamedTargetPorts int
	DocumentJSONBytes       int
	ANFBytes                int
	DocumentJSONTokensEst   int
	ANFTokensEst            int
	Reduction               float64
	TopLevelEntities        int
}

// Result contains the source payload and two encodings of the exact ANF document.
type Result struct {
	Cluster      string
	Namespace    string
	CapturedAt   time.Time
	Source       SourceSnapshot
	SourceJSON   []byte
	View         anfkubernetes.NamespaceView
	Document     *anf.Document
	DocumentJSON []byte
	ANF          string
	Metrics      Metrics
}

// Capture fetches every required list before producing a deterministic snapshot.
func Capture(ctx context.Context, lister Lister, options Options) (*Result, error) {
	if err := validateLabel("cluster", options.Cluster, summaryClusterPattern); err != nil {
		return nil, err
	}
	if err := validateLabel("namespace", options.Namespace, summaryNamespacePattern); err != nil {
		return nil, err
	}

	deployments, err := lister.ListDeployments(ctx, options.Namespace)
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	pods, err := lister.ListPods(ctx, options.Namespace)
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	services, err := lister.ListServices(ctx, options.Namespace)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	jobs, err := lister.ListJobs(ctx, options.Namespace)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	cronJobs, err := lister.ListCronJobs(ctx, options.Namespace)
	if err != nil {
		return nil, fmt.Errorf("list cronjobs: %w", err)
	}

	clock := options.Clock
	if clock == nil {
		clock = time.Now
	}
	capturedAt := clock()
	source := sortedSource(deployments, pods, services, jobs, cronJobs)
	sourceJSON, err := json.Marshal(source)
	if err != nil {
		return nil, fmt.Errorf("marshal Kubernetes source: %w", err)
	}

	view := buildNamespaceView(source, options.Cluster, options.Namespace, capturedAt)
	document := anfkubernetes.Translate(view, capturedAt)
	documentJSON, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("marshal ANF document: %w", err)
	}
	encoded := anf.EncodeToString(document)
	unprojectedPods, omittedContainers, omittedServicePorts, omittedNamedTargetPorts := countProjectionOmissions(source)

	metrics := Metrics{
		SourceBytes:             len(sourceJSON),
		SourceObjects:           countSourceObjects(source),
		ProjectedObjects:        countProjectedObjects(view),
		UnprojectedPods:         unprojectedPods,
		OmittedContainers:       omittedContainers,
		OmittedServicePorts:     omittedServicePorts,
		OmittedNamedTargetPorts: omittedNamedTargetPorts,
		DocumentJSONBytes:       len(documentJSON),
		ANFBytes:                len(encoded),
		DocumentJSONTokensEst:   estimateTokens(len(documentJSON)),
		ANFTokensEst:            estimateTokens(len(encoded)),
		TopLevelEntities:        len(document.Entities),
	}
	if metrics.DocumentJSONBytes > 0 {
		metrics.Reduction = float64(metrics.DocumentJSONBytes-metrics.ANFBytes) / float64(metrics.DocumentJSONBytes) * 100
	}

	return &Result{
		Cluster:      options.Cluster,
		Namespace:    options.Namespace,
		CapturedAt:   capturedAt,
		Source:       source,
		SourceJSON:   sourceJSON,
		View:         view,
		Document:     document,
		DocumentJSON: documentJSON,
		ANF:          encoded,
		Metrics:      metrics,
	}, nil
}

func sortedSource(
	deployments *appsv1.DeploymentList,
	pods *corev1.PodList,
	services *corev1.ServiceList,
	jobs *batchv1.JobList,
	cronJobs *batchv1.CronJobList,
) SourceSnapshot {
	source := SourceSnapshot{
		Deployments: *deployments.DeepCopy(),
		Pods:        *pods.DeepCopy(),
		Services:    *services.DeepCopy(),
		Jobs:        *jobs.DeepCopy(),
		CronJobs:    *cronJobs.DeepCopy(),
	}
	sort.SliceStable(source.Deployments.Items, func(left, right int) bool {
		return objectKey(source.Deployments.Items[left].ObjectMeta) < objectKey(source.Deployments.Items[right].ObjectMeta)
	})
	sort.SliceStable(source.Pods.Items, func(left, right int) bool {
		return objectKey(source.Pods.Items[left].ObjectMeta) < objectKey(source.Pods.Items[right].ObjectMeta)
	})
	sort.SliceStable(source.Services.Items, func(left, right int) bool {
		return objectKey(source.Services.Items[left].ObjectMeta) < objectKey(source.Services.Items[right].ObjectMeta)
	})
	sort.SliceStable(source.Jobs.Items, func(left, right int) bool {
		return objectKey(source.Jobs.Items[left].ObjectMeta) < objectKey(source.Jobs.Items[right].ObjectMeta)
	})
	sort.SliceStable(source.CronJobs.Items, func(left, right int) bool {
		return objectKey(source.CronJobs.Items[left].ObjectMeta) < objectKey(source.CronJobs.Items[right].ObjectMeta)
	})
	return source
}

func objectKey(metadata metav1.ObjectMeta) string {
	return metadata.Namespace + "\x00" + metadata.Name + "\x00" + string(metadata.UID)
}

func buildNamespaceView(source SourceSnapshot, cluster, namespace string, capturedAt time.Time) anfkubernetes.NamespaceView {
	view := anfkubernetes.NamespaceView{Cluster: cluster, Namespace: namespace}
	for _, deployment := range source.Deployments.Items {
		view.Deployments = append(view.Deployments, translateDeployment(deployment, capturedAt))
	}
	for _, service := range source.Services.Items {
		view.Services = append(view.Services, translateService(service))
	}
	for _, job := range source.Jobs.Items {
		view.Jobs = append(view.Jobs, translateJob(job))
	}
	for _, cronJob := range source.CronJobs.Items {
		view.CronJobs = append(view.CronJobs, translateCronJob(cronJob))
	}
	return view
}

func translateDeployment(deployment appsv1.Deployment, capturedAt time.Time) anfkubernetes.Deployment {
	desiredReplicas := int32(0)
	if deployment.Spec.Replicas != nil {
		desiredReplicas = *deployment.Spec.Replicas
	}
	image := ""
	// The proof contract projects only the first container because ANF has one image field.
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		image = deployment.Spec.Template.Spec.Containers[0].Image
	}
	return anfkubernetes.Deployment{
		Name:          deployment.Name,
		Replicas:      desiredReplicas,
		ReadyReplicas: deployment.Status.ReadyReplicas,
		Image:         image,
		Strategy:      string(deployment.Spec.Strategy.Type),
		AgeDays:       ageDays(deployment.CreationTimestamp.Time, capturedAt),
	}
}

func translateService(service corev1.Service) anfkubernetes.Service {
	translated := anfkubernetes.Service{Name: service.Name, Type: string(service.Spec.Type)}
	// The proof contract projects only the first port because ANF has one port pair.
	if len(service.Spec.Ports) == 0 {
		return translated
	}
	if service.Spec.Ports[0].TargetPort.Type != intstr.Int {
		return translated
	}
	translated.Port = service.Spec.Ports[0].Port
	translated.TargetPort = service.Spec.Ports[0].TargetPort.IntVal
	return translated
}

func translateJob(job batchv1.Job) anfkubernetes.Job {
	completed := job.Status.CompletionTime != nil
	succeeded := job.Status.Succeeded > 0
	failed := false
	for _, condition := range job.Status.Conditions {
		if condition.Status != corev1.ConditionTrue {
			continue
		}
		switch condition.Type {
		case batchv1.JobComplete:
			completed = true
			succeeded = true
		case batchv1.JobFailed:
			completed = true
			failed = true
		}
	}
	if failed {
		succeeded = false
	}

	translated := anfkubernetes.Job{Name: job.Name, Completed: completed, Succeeded: succeeded}
	if job.Status.StartTime != nil {
		translated.LastRun = job.Status.StartTime.Time
	}
	if job.Status.StartTime != nil && job.Status.CompletionTime != nil {
		translated.Duration = job.Status.CompletionTime.Sub(job.Status.StartTime.Time)
	}
	return translated
}

func translateCronJob(cronJob batchv1.CronJob) anfkubernetes.CronJob {
	translated := anfkubernetes.CronJob{Name: cronJob.Name, Schedule: cronJob.Spec.Schedule}
	if cronJob.Status.LastScheduleTime != nil {
		translated.LastRun = cronJob.Status.LastScheduleTime.Time
	}
	return translated
}

func countSourceObjects(source SourceSnapshot) int {
	return len(source.Deployments.Items) + len(source.Pods.Items) + len(source.Services.Items) + len(source.Jobs.Items) + len(source.CronJobs.Items)
}

func countProjectedObjects(view anfkubernetes.NamespaceView) int {
	count := len(view.Deployments) + len(view.Services) + len(view.Jobs) + len(view.CronJobs)
	for _, deployment := range view.Deployments {
		count += len(deployment.Pods)
	}
	return count
}

func countProjectionOmissions(source SourceSnapshot) (unprojectedPods, omittedContainers, omittedServicePorts, omittedNamedTargetPorts int) {
	unprojectedPods = len(source.Pods.Items)
	for _, deployment := range source.Deployments.Items {
		if containerCount := len(deployment.Spec.Template.Spec.Containers); containerCount > 1 {
			omittedContainers += containerCount - 1
		}
	}
	for _, service := range source.Services.Items {
		if portCount := len(service.Spec.Ports); portCount > 1 {
			omittedServicePorts += portCount - 1
		}
		if len(service.Spec.Ports) > 0 && service.Spec.Ports[0].TargetPort.Type != intstr.Int {
			omittedNamedTargetPorts++
		}
	}
	return unprojectedPods, omittedContainers, omittedServicePorts, omittedNamedTargetPorts
}

func ageDays(created, capturedAt time.Time) int {
	if created.IsZero() || !capturedAt.After(created) {
		return 0
	}
	return int(capturedAt.Sub(created) / (24 * time.Hour))
}

func estimateTokens(characters int) int {
	if characters == 0 {
		return 0
	}
	tokens := characters / 4
	if tokens < 1 {
		return 1
	}
	return tokens
}

func validateLabel(name, value string, pattern *regexp.Regexp) error {
	if !pattern.MatchString(value) {
		return fmt.Errorf("%s %q is not compatible with the ANF summary contract", name, value)
	}
	return nil
}

// Summary returns the single parseable measurement line used by the demo.
func (result *Result) Summary() string {
	return fmt.Sprintf(
		"ANF context: source=kubernetes/%s scope=namespace:%s source_bytes=%d source_objects=%d projected_objects=%d unprojected_pods=%d omitted_containers=%d omitted_service_ports=%d omitted_named_target_ports=%d document_json_bytes=%d anf_bytes=%d document_json_tokens_est=%d anf_tokens_est=%d reduction=%.1f top_level_entities=%d",
		result.Cluster,
		result.Namespace,
		result.Metrics.SourceBytes,
		result.Metrics.SourceObjects,
		result.Metrics.ProjectedObjects,
		result.Metrics.UnprojectedPods,
		result.Metrics.OmittedContainers,
		result.Metrics.OmittedServicePorts,
		result.Metrics.OmittedNamedTargetPorts,
		result.Metrics.DocumentJSONBytes,
		result.Metrics.ANFBytes,
		result.Metrics.DocumentJSONTokensEst,
		result.Metrics.ANFTokensEst,
		result.Metrics.Reduction,
		result.Metrics.TopLevelEntities,
	)
}

// PreviewLines returns at most limit nonempty ANF lines.
func (result *Result) PreviewLines(limit int) []string {
	if limit <= 0 {
		return nil
	}
	lines := make([]string, 0, limit)
	for _, line := range strings.Split(result.ANF, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
		if len(lines) == limit {
			break
		}
	}
	return lines
}

// WriteArtifact atomically replaces path with a private regular file.
// The local demo assumes the parent directory is trusted and unchanged during this call.
func WriteArtifact(path, content string) (resultErr error) {
	parent := filepath.Dir(path)
	parentInfo, err := os.Stat(parent)
	if err != nil {
		return fmt.Errorf("stat output parent: %w", err)
	}
	if !parentInfo.IsDir() {
		return fmt.Errorf("output parent is not a directory: %s", parent)
	}

	temporary, err := os.CreateTemp(parent, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary artifact: %w", err)
	}
	temporaryPath := temporary.Name()
	closed := false
	defer func() {
		if !closed {
			_ = temporary.Close()
		}
		if resultErr != nil {
			_ = os.Remove(temporaryPath)
		}
	}()

	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("set temporary artifact mode: %w", err)
	}
	if _, err := io.WriteString(temporary, content); err != nil {
		return fmt.Errorf("write temporary artifact: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary artifact: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary artifact: %w", err)
	}
	closed = true
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace artifact: %w", err)
	}
	return nil
}
