package anfsnapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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

// Lister provides the five Kubernetes sources required for a live snapshot.
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

// NewKubernetesLister adapts a client-go clientset to the snapshot Lister.
func NewKubernetesLister(client kubernetes.Interface) Lister {
	return kubernetesLister{client: client}
}

func (l kubernetesLister) ListDeployments(ctx context.Context, namespace string) (*appsv1.DeploymentList, error) {
	return l.client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
}

func (l kubernetesLister) ListPods(ctx context.Context, namespace string) (*corev1.PodList, error) {
	return l.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
}

func (l kubernetesLister) ListServices(ctx context.Context, namespace string) (*corev1.ServiceList, error) {
	return l.client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
}

func (l kubernetesLister) ListJobs(ctx context.Context, namespace string) (*batchv1.JobList, error) {
	return l.client.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
}

func (l kubernetesLister) ListCronJobs(ctx context.Context, namespace string) (*batchv1.CronJobList, error) {
	return l.client.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{})
}

// Options identifies the namespace view and fixes its observation time.
type Options struct {
	Cluster   string
	Namespace string
	Now       time.Time
}

// RawSnapshot is the deterministic comparison input for the five fetched lists.
type RawSnapshot struct {
	Deployments appsv1.DeploymentList `json:"deployments"`
	Pods        corev1.PodList        `json:"pods"`
	Services    corev1.ServiceList    `json:"services"`
	Jobs        batchv1.JobList       `json:"jobs"`
	CronJobs    batchv1.CronJobList   `json:"cronJobs"`
}

// Metrics contains measured byte sizes and estimated token counts.
type Metrics struct {
	RawBytes     int
	ANFBytes     int
	RawTokensEst int
	ANFTokensEst int
	Reduction    float64
	Entities     int
}

// Result contains the observed raw data, translated view, and encoded artifact.
type Result struct {
	Cluster   string
	Namespace string
	Raw       RawSnapshot
	RawJSON   []byte
	View      anfkubernetes.NamespaceView
	Document  *anf.Document
	ANF       string
	Metrics   Metrics
}

// Capture fetches all required sources before producing a live ANF snapshot.
func Capture(ctx context.Context, lister Lister, options Options) (*Result, error) {
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

	raw := RawSnapshot{
		Deployments: *deployments,
		Pods:        *pods,
		Services:    *services,
		Jobs:        *jobs,
		CronJobs:    *cronJobs,
	}
	rawJSON, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal Kubernetes snapshot: %w", err)
	}

	view := buildNamespaceView(raw, options)
	document := anfkubernetes.Translate(view, options.Now)
	encoded := anf.EncodeToString(document)
	rawTokens := estimateTokens(len(rawJSON))
	anfTokens := estimateTokens(len(encoded))
	reduction := 0.0
	if len(rawJSON) > 0 {
		reduction = float64(len(rawJSON)-len(encoded)) / float64(len(rawJSON)) * 100
	}

	return &Result{
		Cluster:   options.Cluster,
		Namespace: options.Namespace,
		Raw:       raw,
		RawJSON:   rawJSON,
		View:      view,
		Document:  document,
		ANF:       encoded,
		Metrics: Metrics{
			RawBytes:     len(rawJSON),
			ANFBytes:     len(encoded),
			RawTokensEst: rawTokens,
			ANFTokensEst: anfTokens,
			Reduction:    reduction,
			Entities:     len(document.Entities),
		},
	}, nil
}

func buildNamespaceView(raw RawSnapshot, options Options) anfkubernetes.NamespaceView {
	view := anfkubernetes.NamespaceView{
		Cluster:   options.Cluster,
		Namespace: options.Namespace,
	}

	for _, deployment := range raw.Deployments.Items {
		view.Deployments = append(view.Deployments, translateDeployment(deployment, options.Now))
	}
	for _, service := range raw.Services.Items {
		view.Services = append(view.Services, translateService(service))
	}
	for _, job := range raw.Jobs.Items {
		view.Jobs = append(view.Jobs, translateJob(job))
	}
	for _, cronJob := range raw.CronJobs.Items {
		view.CronJobs = append(view.CronJobs, translateCronJob(cronJob))
	}

	return view
}

func translateDeployment(deployment appsv1.Deployment, now time.Time) anfkubernetes.Deployment {
	desiredReplicas := int32(0)
	if deployment.Spec.Replicas != nil {
		desiredReplicas = *deployment.Spec.Replicas
	}
	image := ""
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		image = deployment.Spec.Template.Spec.Containers[0].Image
	}

	return anfkubernetes.Deployment{
		Name:          deployment.Name,
		Replicas:      desiredReplicas,
		ReadyReplicas: deployment.Status.ReadyReplicas,
		Image:         image,
		Strategy:      string(deployment.Spec.Strategy.Type),
		AgeDays:       ageDays(deployment.CreationTimestamp.Time, now),
	}
}

func translateService(service corev1.Service) anfkubernetes.Service {
	translated := anfkubernetes.Service{
		Name: service.Name,
		Type: string(service.Spec.Type),
	}
	if len(service.Spec.Ports) == 0 {
		return translated
	}

	translated.Port = service.Spec.Ports[0].Port
	if service.Spec.Ports[0].TargetPort.Type == intstr.Int {
		translated.TargetPort = service.Spec.Ports[0].TargetPort.IntVal
	}
	return translated
}

func translateJob(job batchv1.Job) anfkubernetes.Job {
	completed := job.Status.CompletionTime != nil
	succeeded := job.Status.Succeeded > 0
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
		}
	}

	translated := anfkubernetes.Job{
		Name:      job.Name,
		Completed: completed,
		Succeeded: succeeded,
	}
	if job.Status.StartTime != nil {
		translated.LastRun = job.Status.StartTime.Time
	}
	if job.Status.StartTime != nil && job.Status.CompletionTime != nil {
		translated.Duration = job.Status.CompletionTime.Sub(job.Status.StartTime.Time)
	}
	return translated
}

func translateCronJob(cronJob batchv1.CronJob) anfkubernetes.CronJob {
	translated := anfkubernetes.CronJob{
		Name:     cronJob.Name,
		Schedule: cronJob.Spec.Schedule,
	}
	if cronJob.Status.LastScheduleTime != nil {
		translated.LastRun = cronJob.Status.LastScheduleTime.Time
	}
	return translated
}

func ageDays(created, now time.Time) int {
	if created.IsZero() || !now.After(created) {
		return 0
	}
	return int(now.Sub(created) / (24 * time.Hour))
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

// Summary returns the single parseable measurement line used by the demo.
func (result *Result) Summary() string {
	return fmt.Sprintf(
		"ANF context: source=kubernetes/%s scope=namespace:%s raw_bytes=%d anf_bytes=%d raw_tokens_est=%d anf_tokens_est=%d reduction=%.1f entities=%d",
		result.Cluster,
		result.Namespace,
		result.Metrics.RawBytes,
		result.Metrics.ANFBytes,
		result.Metrics.RawTokensEst,
		result.Metrics.ANFTokensEst,
		result.Metrics.Reduction,
		result.Metrics.Entities,
	)
}

// PreviewLines returns up to limit nonempty lines from the ANF artifact.
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

// WriteArtifact writes the ANF document without creating its parent directory.
func WriteArtifact(path, content string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}

	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
