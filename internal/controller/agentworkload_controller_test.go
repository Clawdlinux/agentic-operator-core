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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	agenticv1alpha1 "github.com/shreyansh/agentic-operator/api/v1alpha1"
)

var _ = Describe("AgentWorkloadReconciler", func() {
	const (
		resourceName = "test-resource"
		namespace    = "default"
	)

	ctx := context.Background()
	namespacedName := types.NamespacedName{Name: resourceName, Namespace: namespace}

	newReconciler := func() *AgentWorkloadReconciler {
		return &AgentWorkloadReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
	}

	AfterEach(func() {
		resource := &agenticv1alpha1.AgentWorkload{}
		err := k8sClient.Get(ctx, namespacedName, resource)
		if errors.IsNotFound(err) {
			return
		}
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
	})

	It("marks workload as Failed when MCP status retrieval fails", func() {
		endpoint := "http://127.0.0.1:0"
		objective := "optimize cluster utilization"

		resource := &agenticv1alpha1.AgentWorkload{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: namespace,
			},
			Spec: agenticv1alpha1.AgentWorkloadSpec{
				MCPServerEndpoint: &endpoint,
				Objective:         &objective,
			},
		}
		Expect(k8sClient.Create(ctx, resource)).To(Succeed())

		_, err := newReconciler().Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) {
			updated := &agenticv1alpha1.AgentWorkload{}
			g.Expect(k8sClient.Get(ctx, namespacedName, updated)).To(Succeed())
			g.Expect(updated.Status.Phase).To(Equal("Failed"))
		}).Should(Succeed())
	})

	It("ignores missing workload requests", func() {
		_, err := newReconciler().Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
		Expect(err).NotTo(HaveOccurred())
	})
})
