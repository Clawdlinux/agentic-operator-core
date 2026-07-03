/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package runtime

import (
	"context"
	"fmt"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// PodRuntimeAdapter runs an AgentWorkload as a single bring-your-own Kubernetes
// Pod. It exists to prove the RuntimeAdapter contract is not Argo-shaped: a
// plain pod is the simplest possible runtime, yet it is governed identically
// because its pod carries the sandbox label the admission webhook keys on.
//
// Image is the container image used for the execution pod. It is required;
// the adapter fails closed rather than creating an empty pod.
type PodRuntimeAdapter struct {
	Client client.Client
	Scheme *runtime.Scheme
	Image  string
}

var _ RuntimeAdapter = (*PodRuntimeAdapter)(nil)

func (a *PodRuntimeAdapter) Capabilities() RuntimeCapabilities {
	// A single pod is one step: no DAG, no resume, no artifact graph.
	return RuntimeCapabilities{
		SupportsDAG:        false,
		SupportsResume:     false,
		SupportsArtifacts:  false,
		SupportsCostReport: false,
	}
}

func (a *PodRuntimeAdapter) Execute(ctx context.Context, workload *agenticv1alpha1.AgentWorkload) (*ExecutionStatus, error) {
	if a.Image == "" {
		return nil, fmt.Errorf("pod adapter execute: no execution image configured")
	}
	log := logf.FromContext(ctx)

	pod := buildGovernedPod(workload, a.Image)
	if a.Scheme != nil {
		if err := controllerutil.SetControllerReference(workload, pod, a.Scheme); err != nil {
			return nil, fmt.Errorf("pod adapter execute: set owner ref: %w", err)
		}
	}

	if err := a.Client.Create(ctx, pod); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("pod adapter execute: create pod: %w", err)
		}
	}

	log.Info("Pod adapter: governed pod created", "name", pod.GetName())
	return &ExecutionStatus{
		Phase:     "Pending",
		Name:      pod.GetName(),
		Namespace: pod.GetNamespace(),
		UID:       string(pod.GetUID()),
	}, nil
}

func (a *PodRuntimeAdapter) Status(ctx context.Context, workload *agenticv1alpha1.AgentWorkload) (*ExecutionStatus, error) {
	pod := &corev1.Pod{}
	key := types.NamespacedName{Name: PodName(workload), Namespace: workload.GetNamespace()}
	if err := a.Client.Get(ctx, key, pod); err != nil {
		return nil, fmt.Errorf("pod adapter status: %w", err)
	}

	return &ExecutionStatus{
		Phase:     normalizePodPhase(pod.Status.Phase),
		Name:      pod.GetName(),
		Namespace: pod.GetNamespace(),
		UID:       string(pod.GetUID()),
	}, nil
}

func (a *PodRuntimeAdapter) Cleanup(ctx context.Context, workload *agenticv1alpha1.AgentWorkload) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: PodName(workload), Namespace: workload.GetNamespace()},
	}
	if err := a.Client.Delete(ctx, pod); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("pod adapter cleanup: %w", err)
	}
	return nil
}

// PodName returns the deterministic execution pod name for a workload.
func PodName(workload *agenticv1alpha1.AgentWorkload) string {
	return workload.GetName() + "-agent"
}

// buildGovernedPod constructs the execution pod for a workload. It is a pure
// function (no client, no cluster) so the governance contract is unit-testable.
//
// The critical line is the sandbox label: the admission webhook injects the
// gVisor RuntimeClass onto any pod carrying it. The adapter does NOT set
// RuntimeClassName itself; governance is applied by the platform, identically
// for every runtime.
func buildGovernedPod(workload *agenticv1alpha1.AgentWorkload, image string) *corev1.Pod {
	labels := governanceLabels(workload)
	labels["agentic.clawdlinux.org/runtime"] = "pod"
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PodName(workload),
			Namespace: workload.GetNamespace(),
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:  "agent",
					Image: image,
				},
			},
		},
	}
}

// normalizePodPhase maps a Kubernetes pod phase onto the normalized
// ExecutionStatus phase vocabulary shared by every adapter.
func normalizePodPhase(phase corev1.PodPhase) string {
	switch phase {
	case corev1.PodPending:
		return "Pending"
	case corev1.PodRunning:
		return "Running"
	case corev1.PodSucceeded:
		return "Succeeded"
	case corev1.PodFailed:
		return "Failed"
	default:
		return "Error"
	}
}
