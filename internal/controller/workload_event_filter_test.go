/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
)

func TestWorkloadEventFilter(t *testing.T) {
	base := func() *agenticv1alpha1.AgentWorkload {
		return &agenticv1alpha1.AgentWorkload{ObjectMeta: metav1.ObjectMeta{
			Name: "w", Namespace: "ns", Generation: 1, ResourceVersion: "1",
			Annotations: map[string]string{"a": "1"}, Labels: map[string]string{"l": "1"},
		}}
	}
	now := metav1.Now()
	tests := []struct {
		name   string
		mutate func(*agenticv1alpha1.AgentWorkload)
		want   bool
	}{
		{"status only", func(w *agenticv1alpha1.AgentWorkload) {
			w.ResourceVersion = "2"
			w.Status.Phase = "Completed"
		}, false},
		{"finalizer only", func(w *agenticv1alpha1.AgentWorkload) {
			w.Finalizers = []string{AgentWorkloadFinalizer}
		}, false},
		{"spec change", func(w *agenticv1alpha1.AgentWorkload) { w.Generation = 2 }, true},
		{"approval annotation", func(w *agenticv1alpha1.AgentWorkload) {
			w.Annotations["clawdlinux.org/approval-decision"] = "approve"
		}, true},
		{"label change", func(w *agenticv1alpha1.AgentWorkload) { w.Labels["l"] = "2" }, true},
		{"deletion", func(w *agenticv1alpha1.AgentWorkload) { w.DeletionTimestamp = &now }, true},
	}
	f := workloadEventFilter()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old, upd := base(), base()
			tt.mutate(upd)
			if got := f.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: upd}); got != tt.want {
				t.Fatalf("Update() = %v, want %v", got, tt.want)
			}
		})
	}
	if !f.Create(event.CreateEvent{Object: base()}) {
		t.Fatal("Create must reconcile")
	}
}
