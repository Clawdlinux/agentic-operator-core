package stress_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/internal/controller"
)

func TestController_10ConcurrentWorkloads(t *testing.T) {
	ctx := context.Background()
	scheme := newStressTestScheme(t)
	server := newStressMockMCPServer(t)
	defer server.Close()

	start := time.Now()
	var wg sync.WaitGroup
	errCh := make(chan error, 10)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("stress-workload-%02d", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if recovered := recover(); recovered != nil {
					errCh <- fmt.Errorf("%s panic: %v", name, recovered)
				}
			}()

			workload := newStressWorkload(name, server.URL)
			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
				WithObjects(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: workload.Namespace}}, workload).
				Build()

			reconciler := &controller.AgentWorkloadReconciler{Client: k8sClient, Scheme: scheme}
			_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}})
			if err != nil {
				errCh <- fmt.Errorf("%s reconcile: %w", workload.Name, err)
				return
			}

			updated := &agenticv1alpha1.AgentWorkload{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}, updated); err != nil {
				errCh <- fmt.Errorf("%s fetch updated: %w", workload.Name, err)
				return
			}
			if updated.Status.Phase == "" {
				errCh <- fmt.Errorf("%s phase is empty", workload.Name)
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("reconciled 10 workloads in %s", time.Since(start))
}

func TestController_RapidCreateDelete(t *testing.T) {
	ctx := context.Background()
	scheme := newStressTestScheme(t)
	server := newStressMockMCPServer(t)
	defer server.Close()

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
		WithObjects(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}).
		Build()
	reconciler := &controller.AgentWorkloadReconciler{Client: k8sClient, Scheme: scheme}

	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("rapid-workload-%02d", i)
		workload := newStressWorkload(name, server.URL)
		if err := k8sClient.Create(ctx, workload); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}

		req := ctrl.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: workload.Namespace}}
		if _, err := reconciler.Reconcile(ctx, req); err != nil {
			t.Fatalf("initial reconcile %s: %v", name, err)
		}

		updated := &agenticv1alpha1.AgentWorkload{}
		if err := k8sClient.Get(ctx, req.NamespacedName, updated); err != nil {
			t.Fatalf("fetch %s after reconcile: %v", name, err)
		}
		if !hasFinalizer(updated.Finalizers, controller.AgentWorkloadFinalizer) {
			t.Fatalf("%s missing finalizer after reconcile: %v", name, updated.Finalizers)
		}

		if err := k8sClient.Delete(ctx, updated); err != nil {
			t.Fatalf("delete %s: %v", name, err)
		}
		if _, err := reconciler.Reconcile(ctx, req); err != nil {
			t.Fatalf("delete reconcile %s: %v", name, err)
		}

		remaining := &agenticv1alpha1.AgentWorkload{}
		err := k8sClient.Get(ctx, req.NamespacedName, remaining)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			t.Fatalf("fetch %s after delete reconcile: %v", name, err)
		}
		t.Fatalf("%s still exists after delete reconcile with finalizers %v", name, remaining.Finalizers)
	}
}

func newStressWorkload(name, endpoint string) *agenticv1alpha1.AgentWorkload {
	objective := "optimize cluster resources"
	policy := "strict"
	return &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", UID: types.UID(name + "-uid")},
		Spec: agenticv1alpha1.AgentWorkloadSpec{
			MCPServerEndpoint: &endpoint,
			Objective:         &objective,
			OPAPolicy:         &policy,
		},
	}
}

func newStressTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := agenticv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add agentic scheme: %v", err)
	}
	return scheme
}

func newStressMockMCPServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/call_tool" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Tool string `json:"tool"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		resp := map[string]interface{}{"tool": req.Tool, "success": true}
		switch req.Tool {
		case "get_status":
			resp["result"] = map[string]interface{}{"status": "healthy", "cluster_health": 90.0}
		case "propose_action":
			resp["result"] = map[string]interface{}{
				"action":      "optimize",
				"description": "Tune resource requests based on observed usage",
				"confidence":  0.98,
			}
		case "execute_action":
			resp["result"] = map[string]interface{}{"executed": true}
		default:
			resp["success"] = false
			resp["error"] = "unknown tool"
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func hasFinalizer(finalizers []string, want string) bool {
	for _, finalizer := range finalizers {
		if finalizer == want {
			return true
		}
	}
	return false
}
