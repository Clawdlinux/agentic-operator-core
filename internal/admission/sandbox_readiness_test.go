package admission

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestKubernetesSandboxReadinessChecker(t *testing.T) {
	tests := []struct {
		name         string
		runtimeClass *nodev1.RuntimeClass
		nodes        []*corev1.Node
		wantReady    bool
		wantReason   string
	}{
		{name: "runtime class missing", wantReason: SandboxReadinessRuntimeClassMissing},
		{name: "no ready node", runtimeClass: runtimeClass("gvisor", nil), nodes: []*corev1.Node{notReadyNode("node-1", map[string]string{gVisorReadyNodeLabel: gVisorReadyNodeValue})}, wantReason: SandboxReadinessNoReadyNode},
		{name: "ready convention labelled node", runtimeClass: runtimeClass("gvisor", nil), nodes: []*corev1.Node{readyNode("node-1", map[string]string{gVisorReadyNodeLabel: gVisorReadyNodeValue})}, wantReady: true, wantReason: SandboxReadinessVerified},
		{name: "runtime class selector matches ready node", runtimeClass: runtimeClass("gvisor", map[string]string{"sandbox": "gvisor"}), nodes: []*corev1.Node{readyNode("node-1", map[string]string{"sandbox": "gvisor"})}, wantReady: true, wantReason: SandboxReadinessVerified},
		{name: "runtime class selector does not match node", runtimeClass: runtimeClass("gvisor", map[string]string{"sandbox": "gvisor"}), nodes: []*corev1.Node{readyNode("node-1", map[string]string{"sandbox": "kata"})}, wantReason: SandboxReadinessNoReadyNode},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := readinessClient(t, test.runtimeClass, test.nodes...)
			checker := NewKubernetesSandboxReadinessChecker(client, "gvisor")
			ready, reason, err := checker.IsReady(context.Background())
			if err != nil {
				t.Fatalf("IsReady error = %v", err)
			}
			if ready != test.wantReady || reason != test.wantReason {
				t.Fatalf("IsReady = (%t, %q), want (%t, %q)", ready, reason, test.wantReady, test.wantReason)
			}
		})
	}
}

func TestKubernetesSandboxReadinessCheckerCachesResult(t *testing.T) {
	now := time.Date(2026, time.September, 3, 0, 0, 0, 0, time.UTC)
	client := readinessClient(t, runtimeClass("gvisor", nil), readyNode("node-1", map[string]string{gVisorReadyNodeLabel: gVisorReadyNodeValue}))
	checker := NewKubernetesSandboxReadinessChecker(client, "gvisor")
	checker.now = func() time.Time { return now }

	ready, _, err := checker.IsReady(context.Background())
	if err != nil || !ready {
		t.Fatalf("initial IsReady = (%t, %v), want (true, nil)", ready, err)
	}
	if err := client.Delete(context.Background(), runtimeClass("gvisor", nil)); err != nil {
		t.Fatal(err)
	}
	ready, _, err = checker.IsReady(context.Background())
	if err != nil || !ready {
		t.Fatalf("cached IsReady = (%t, %v), want cached true", ready, err)
	}
	now = now.Add(defaultSandboxReadinessTTL)
	ready, reason, err := checker.IsReady(context.Background())
	if err != nil || ready || reason != SandboxReadinessRuntimeClassMissing {
		t.Fatalf("expired IsReady = (%t, %q, %v)", ready, reason, err)
	}
}

func TestKubernetesSandboxReadinessCheckerDoesNotCacheAPIErrors(t *testing.T) {
	checker := NewKubernetesSandboxReadinessChecker(failingSandboxReader{}, "gvisor")
	for attempt := 0; attempt < 2; attempt++ {
		ready, reason, err := checker.IsReady(context.Background())
		if ready || reason != SandboxReadinessCheckFailed || !errors.Is(err, errSandboxAPI) {
			t.Fatalf("attempt %d IsReady = (%t, %q, %v)", attempt+1, ready, reason, err)
		}
		if !checker.cached.checkedAt.IsZero() {
			t.Fatalf("attempt %d cached API failure", attempt+1)
		}
	}
}

func readinessClient(t *testing.T, runtimeClass *nodev1.RuntimeClass, nodes ...*corev1.Node) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := nodev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	objects := make([]runtime.Object, 0, len(nodes)+1)
	if runtimeClass != nil {
		objects = append(objects, runtimeClass)
	}
	for _, node := range nodes {
		objects = append(objects, node)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()
}

func runtimeClass(name string, selector map[string]string) *nodev1.RuntimeClass {
	return &nodev1.RuntimeClass{ObjectMeta: metav1.ObjectMeta{Name: name}, Handler: "runsc", Scheduling: &nodev1.Scheduling{NodeSelector: selector}}
}

func notReadyNode(name string, labels map[string]string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
}

func readyNode(name string, labels map[string]string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Status:     corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}},
	}
}

var errSandboxAPI = errors.New("sandbox API unavailable")

type failingSandboxReader struct{}

func (failingSandboxReader) Get(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error {
	return errSandboxAPI
}

func (failingSandboxReader) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return errSandboxAPI
}
