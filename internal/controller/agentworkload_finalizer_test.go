/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package controller

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	agenticv1alpha1 "github.com/shreyansh/agentic-operator/api/v1alpha1"
)

// Regression coverage for P0 launch fix:
// AgentWorkload must be issued a finalizer so cross-namespace Argo Workflows
// are explicitly cleaned up before the resource is deleted.
var _ = Describe("AgentWorkload finalizer (P0 launch fix)", func() {
	const namespace = "default"
	ctx := context.Background()

	var (
		resourceName string
		nn           types.NamespacedName
		counter      int
	)

	BeforeEach(func() {
		counter++
		resourceName = fmt.Sprintf("finalizer-test-%d", counter)
		nn = types.NamespacedName{Name: resourceName, Namespace: namespace}
	})

	AfterEach(func() {
		got := &agenticv1alpha1.AgentWorkload{}
		if err := k8sClient.Get(ctx, nn, got); err == nil {
			// Drop finalizers so the resource can actually be deleted in test teardown.
			if len(got.Finalizers) > 0 {
				got.Finalizers = nil
				_ = k8sClient.Update(ctx, got)
			}
			_ = k8sClient.Delete(ctx, got)
		}
		// Wait for object to actually be gone so the next test's Create doesn't race.
		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, nn, &agenticv1alpha1.AgentWorkload{}))
		}).Should(BeTrue())
	})

	It("adds the AgentWorkload finalizer on first reconcile", func() {
		endpoint := "https://127.0.0.1:0"
		objective := "test"
		resource := &agenticv1alpha1.AgentWorkload{
			ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: namespace},
			Spec: agenticv1alpha1.AgentWorkloadSpec{
				MCPServerEndpoint: &endpoint,
				Objective:         &objective,
			},
		}
		Expect(k8sClient.Create(ctx, resource)).To(Succeed())

		reconciler := &AgentWorkloadReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) {
			updated := &agenticv1alpha1.AgentWorkload{}
			g.Expect(k8sClient.Get(ctx, nn, updated)).To(Succeed())
			g.Expect(updated.Finalizers).To(ContainElement(AgentWorkloadFinalizer))
		}).Should(Succeed())
	})

	It("removes the finalizer once cleanup completes (no Argo workflow recorded)", func() {
		endpoint := "https://127.0.0.1:0"
		objective := "test"
		resource := &agenticv1alpha1.AgentWorkload{
			ObjectMeta: metav1.ObjectMeta{
				Name:       resourceName,
				Namespace:  namespace,
				Finalizers: []string{AgentWorkloadFinalizer},
			},
			Spec: agenticv1alpha1.AgentWorkloadSpec{
				MCPServerEndpoint: &endpoint,
				Objective:         &objective,
			},
		}
		Expect(k8sClient.Create(ctx, resource)).To(Succeed())

		// Issue delete: object enters deletion timestamp state but stays around due to finalizer.
		Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

		reconciler := &AgentWorkloadReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() bool {
			err := k8sClient.Get(ctx, nn, &agenticv1alpha1.AgentWorkload{})
			return apierrors.IsNotFound(err)
		}).Should(BeTrue())
	})
})
