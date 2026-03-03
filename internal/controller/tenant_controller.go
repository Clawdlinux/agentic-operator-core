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
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	agenticv1alpha1 "github.com/shreyansh/agentic-operator/api/v1alpha1"
)

// TenantReconciler reconciles a Tenant object
type TenantReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=agentic.clawdlinux.org,resources=tenants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=agentic.clawdlinux.org,resources=tenants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=agentic.clawdlinux.org,resources=tenants/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=create;get;list;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=create;get;list;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=create;get;list;patch
// +kubebuilder:rbac:groups="",resources=resourcequotas,verbs=create;get;list;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=create;get;list;patch

// Reconcile implements the reconciliation loop for Tenant provisioning
func (r *TenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the Tenant
	var tenant agenticv1alpha1.Tenant
	if err := r.Get(ctx, req.NamespacedName, &tenant); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Tenant deleted", "tenant", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch Tenant")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling Tenant", "name", tenant.Name, "namespace", tenant.Spec.Namespace)

	// Update phase to Provisioning
	meta.SetStatusCondition(&tenant.Status.Conditions, metav1.Condition{
		Type:               "Provisioning",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: tenant.Generation,
		Reason:             "ProvisioningStarted",
		Message:            "Tenant provisioning in progress",
	})
	tenant.Status.Phase = "Provisioning"

	// Step 1: Create Namespace
	if !tenant.Status.NamespaceCreated {
		if err := r.createNamespace(ctx, &tenant); err != nil {
			log.Error(err, "failed to create namespace")
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		tenant.Status.NamespaceCreated = true
		log.Info("Namespace created", "namespace", tenant.Spec.Namespace)
	}

	// Step 2: Provision Secrets
	if !tenant.Status.SecretsProvisioned {
		if err := r.provisionSecrets(ctx, &tenant); err != nil {
			log.Error(err, "failed to provision secrets")
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		tenant.Status.SecretsProvisioned = true
		log.Info("Secrets provisioned", "namespace", tenant.Spec.Namespace)
	}

	// Step 3: Configure RBAC
	if !tenant.Status.RBACConfigured {
		if err := r.configureRBAC(ctx, &tenant); err != nil {
			log.Error(err, "failed to configure RBAC")
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		tenant.Status.RBACConfigured = true
		log.Info("RBAC configured", "namespace", tenant.Spec.Namespace)
	}

	// Step 4: Enforce Quotas
	if !tenant.Status.QuotasEnforced {
		if err := r.enforceQuotas(ctx, &tenant); err != nil {
			log.Error(err, "failed to enforce quotas")
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		tenant.Status.QuotasEnforced = true
		log.Info("Quotas enforced", "namespace", tenant.Spec.Namespace)
	}

	// All provisioning complete
	meta.SetStatusCondition(&tenant.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionTrue,
		Reason:  "ProvisioningComplete",
		Message: "Tenant fully provisioned and ready",
	})
	tenant.Status.Phase = "Active"
	tenant.Status.LastReconciliation = &metav1.Time{Time: time.Now()}

	if err := r.Status().Update(ctx, &tenant); err != nil {
		log.Error(err, "failed to update status")
		return ctrl.Result{}, err
	}

	log.Info("Tenant provisioned successfully", "tenant", tenant.Name)
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *TenantReconciler) createNamespace(ctx context.Context, tenant *agenticv1alpha1.Tenant) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: tenant.Spec.Namespace,
			Labels: map[string]string{
				"agentic-tenant":   "true",
				"agentic-customer": tenant.Name,
				"managed-by":       "agentic-operator",
			},
		},
	}

	err := r.Create(ctx, ns)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (r *TenantReconciler) provisionSecrets(ctx context.Context, tenant *agenticv1alpha1.Tenant) error {
	// Copy provider secrets from agentic-system to tenant namespace
	for _, provider := range tenant.Spec.Providers {
		secretName := fmt.Sprintf("%s-token", provider)

		// Read source secret from agentic-system
		srcSecret := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{
			Name:      secretName,
			Namespace: "agentic-system",
		}, srcSecret); err != nil {
			return fmt.Errorf("failed to read source secret %s: %w", secretName, err)
		}

		// Create copy in tenant namespace
		dstSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: tenant.Spec.Namespace,
				Labels: map[string]string{
					"agentic-tenant": tenant.Name,
				},
			},
			Type: srcSecret.Type,
			Data: srcSecret.Data,
		}

		err := r.Create(ctx, dstSecret)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create secret %s: %w", secretName, err)
		}
	}
	return nil
}

func (r *TenantReconciler) configureRBAC(ctx context.Context, tenant *agenticv1alpha1.Tenant) error {
	// Create service account
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-agent", tenant.Name),
			Namespace: tenant.Spec.Namespace,
		},
	}
	if err := r.Create(ctx, sa); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	// Create role for workload management
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-workload-manager", tenant.Name),
			Namespace: tenant.Spec.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"agentic.clawdlinux.org"},
				Resources: []string{"agentworkloads"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
	if err := r.Create(ctx, role); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	// Create role binding
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-workload-binding", tenant.Name),
			Namespace: tenant.Spec.Namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     fmt.Sprintf("%s-workload-manager", tenant.Name),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      fmt.Sprintf("%s-agent", tenant.Name),
				Namespace: tenant.Spec.Namespace,
			},
		},
	}
	if err := r.Create(ctx, rb); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func (r *TenantReconciler) enforceQuotas(ctx context.Context, tenant *agenticv1alpha1.Tenant) error {
	quotas := tenant.Spec.Quotas

	rq := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-quota", tenant.Name),
			Namespace: tenant.Spec.Namespace,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{},
		},
	}

	if quotas.CPULimit != "" {
		rq.Spec.Hard[corev1.ResourceCPU] = resource.MustParse(quotas.CPULimit)
	}
	if quotas.MemoryLimit != "" {
		rq.Spec.Hard[corev1.ResourceMemory] = resource.MustParse(quotas.MemoryLimit)
	}
	if quotas.MaxWorkloads > 0 {
		rq.Spec.Hard[corev1.ResourcePods] = *resource.NewQuantity(int64(quotas.MaxWorkloads), resource.DecimalSI)
	}

	err := r.Create(ctx, rq)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&agenticv1alpha1.Tenant{}).
		Owns(&corev1.Namespace{}).
		Owns(&corev1.Secret{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&corev1.ResourceQuota{}).
		Named("tenant").
		Complete(r)
}
