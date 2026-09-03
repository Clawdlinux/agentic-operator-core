package admission

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	SandboxReadinessVerified            = "Verified"
	SandboxReadinessRuntimeClassMissing = "RuntimeClassMissing"
	SandboxReadinessNoReadyNode         = "NoReadyNode"
	SandboxReadinessCheckFailed         = "SandboxReadinessCheckFailed"
	gVisorReadyNodeLabel                = "agentic.clawdlinux.org/gvisor-ready"
	gVisorReadyNodeValue                = "true"
	defaultSandboxReadinessTTL          = 30 * time.Second
)

// SandboxReadinessChecker reports whether a RuntimeClass has a ready node that
// can run sandboxed workloads.
type SandboxReadinessChecker interface {
	IsReady(ctx context.Context) (ready bool, reason string, err error)
}

type sandboxReadinessResult struct {
	ready     bool
	reason    string
	err       error
	checkedAt time.Time
}

// KubernetesSandboxReadinessChecker checks RuntimeClass and Node readiness
// through an uncached API reader. Results are cached because admission runs on
// every matching pod creation.
type KubernetesSandboxReadinessChecker struct {
	reader           client.Reader
	runtimeClassName string
	ttl              time.Duration
	now              func() time.Time

	mu     sync.Mutex
	cached sandboxReadinessResult
}

func NewKubernetesSandboxReadinessChecker(reader client.Reader, runtimeClassName string) *KubernetesSandboxReadinessChecker {
	return &KubernetesSandboxReadinessChecker{
		reader:           reader,
		runtimeClassName: runtimeClassName,
		ttl:              defaultSandboxReadinessTTL,
		now:              time.Now,
	}
}

func (c *KubernetesSandboxReadinessChecker) IsReady(ctx context.Context) (bool, string, error) {
	c.mu.Lock()
	if !c.cached.checkedAt.IsZero() && c.now().Sub(c.cached.checkedAt) < c.ttl {
		result := c.cached
		c.mu.Unlock()
		return result.ready, result.reason, result.err
	}
	c.mu.Unlock()

	result := c.check(ctx)
	if ctx.Err() == nil && result.err == nil {
		c.mu.Lock()
		result.checkedAt = c.now()
		c.cached = result
		c.mu.Unlock()
	}
	return result.ready, result.reason, result.err
}

func (c *KubernetesSandboxReadinessChecker) check(ctx context.Context) sandboxReadinessResult {
	if c.reader == nil {
		return sandboxReadinessResult{reason: SandboxReadinessCheckFailed, err: fmt.Errorf("sandbox readiness: nil API reader")}
	}

	runtimeClass := &nodev1.RuntimeClass{}
	if err := c.reader.Get(ctx, client.ObjectKey{Name: c.runtimeClassName}, runtimeClass); err != nil {
		if apierrors.IsNotFound(err) {
			return sandboxReadinessResult{reason: SandboxReadinessRuntimeClassMissing}
		}
		return sandboxReadinessResult{reason: SandboxReadinessCheckFailed, err: fmt.Errorf("sandbox readiness: get RuntimeClass %q: %w", c.runtimeClassName, err)}
	}

	nodes := &corev1.NodeList{}
	if err := c.reader.List(ctx, nodes); err != nil {
		return sandboxReadinessResult{reason: SandboxReadinessCheckFailed, err: fmt.Errorf("sandbox readiness: list nodes: %w", err)}
	}
	for index := range nodes.Items {
		node := &nodes.Items[index]
		if !isReadyNode(node) {
			continue
		}
		if matchesRuntimeClassNode(node, runtimeClass) {
			return sandboxReadinessResult{ready: true, reason: SandboxReadinessVerified}
		}
	}
	return sandboxReadinessResult{reason: SandboxReadinessNoReadyNode}
}

func isReadyNode(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func matchesRuntimeClassNode(node *corev1.Node, runtimeClass *nodev1.RuntimeClass) bool {
	selector := runtimeClass.Scheduling
	if selector == nil || len(selector.NodeSelector) == 0 {
		return node.Labels[gVisorReadyNodeLabel] == gVisorReadyNodeValue
	}
	for key, value := range selector.NodeSelector {
		if node.Labels[key] != value {
			return false
		}
	}
	return true
}
