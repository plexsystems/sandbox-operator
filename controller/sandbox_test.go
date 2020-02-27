// +build !integration

package controller

import (
	"context"
	"log"
	"testing"

	operatorsv1alpha1 "github.com/plexsystems/sandbox-operator/apis/operators/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestSandboxController_ByDefault_CreatesSandbox(t *testing.T) {
	ctx := context.TODO()

	s := scheme.Scheme
	s.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.Sandbox{})

	fakeClient := fake.NewFakeClientWithScheme(s)
	r := ReconcileSandbox{
		client:         fakeClient,
		scheme:         s,
		subjectsClient: &DefaultSubjects{},
	}

	sandbox := operatorsv1alpha1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}

	if err := r.client.Create(ctx, &sandbox); err != nil {
		t.Fatalf("create sandbox: %v", err)
	}

	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: sandbox.Name,
		},
	}

	if _, err := r.Reconcile(request); err != nil {
		log.Fatalf("reconcile sandbox: %v", err)
	}

	namespace := getNamespace(sandbox)
	if err := r.client.Get(ctx, types.NamespacedName{Name: namespace.Name}, &corev1.Namespace{}); err != nil {
		t.Errorf("expected Namespace to be created but it was not: %v", err)
	}

	role := getRole(sandbox)
	if err := r.client.Get(ctx, types.NamespacedName{Name: role.Name, Namespace: role.Namespace}, &rbacv1.Role{}); err != nil {
		t.Errorf("expected Role to be created but it was not: %v", err)
	}

	roleBinding := getRoleBinding(sandbox)
	if err := r.client.Get(ctx, types.NamespacedName{Name: roleBinding.Name, Namespace: roleBinding.Namespace}, &rbacv1.RoleBinding{}); err != nil {
		t.Errorf("expected RoleBinding to be created but it was not: %v", err)
	}

	clusterRole := getClusterRole(sandbox)
	if err := r.client.Get(ctx, types.NamespacedName{Name: clusterRole.Name}, &rbacv1.ClusterRole{}); err != nil {
		t.Errorf("expected ClusterRole to be created but it was not: %v", err)
	}

	clusterRoleBinding := getClusterRoleBinding(sandbox)
	if err := r.client.Get(ctx, types.NamespacedName{Name: clusterRoleBinding.Name}, &rbacv1.ClusterRoleBinding{}); err != nil {
		t.Errorf("expected ClusterRoleBinding to be created but it was not: %v", err)
	}

	resourceQuota := getResourceQuota(sandbox)
	if err := r.client.Get(ctx, types.NamespacedName{Name: resourceQuota.Name, Namespace: resourceQuota.Namespace}, &corev1.ResourceQuota{}); err != nil {
		t.Errorf("expected ResourceQuota to be created but it was not: %v", err)
	}
}

func TestSandboxController_AddOwner_UpdatesRoleAndClusterRoleBindings(t *testing.T) {
	ctx := context.TODO()

	s := scheme.Scheme
	s.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.Sandbox{})

	fakeClient := fake.NewFakeClientWithScheme(s)
	r := ReconcileSandbox{
		client:         fakeClient,
		scheme:         s,
		subjectsClient: DefaultSubjects{},
	}

	sandbox := operatorsv1alpha1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: operatorsv1alpha1.SandboxSpec{
			Owners: []string{"foo"},
		},
	}

	if err := r.client.Create(ctx, &sandbox); err != nil {
		t.Fatalf("create sandbox: %v", err)
	}

	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: sandbox.Name,
		},
	}

	if _, err := r.Reconcile(request); err != nil {
		log.Fatalf("reconcile sandbox: %v", err)
	}

	roleBinding := getRoleBinding(sandbox)
	clusterRoleBinding := getClusterRoleBinding(sandbox)

	var foundRoleBinding rbacv1.RoleBinding
	var foundClusterRoleBinding rbacv1.ClusterRoleBinding
	if err := r.client.Get(ctx, types.NamespacedName{Name: roleBinding.Name, Namespace: roleBinding.Namespace}, &foundRoleBinding); err != nil {
		t.Fatalf("expected RoleBinding to exist but it does not: %v", err)
	}

	if err := r.client.Get(ctx, types.NamespacedName{Name: clusterRoleBinding.Name}, &foundClusterRoleBinding); err != nil {
		t.Fatalf("expected ClusterRoleBinding to exist but it does not: %v", err)
	}

	if foundRoleBinding.Subjects[0].Name != "foo" {
		t.Errorf("expected subject to be added to RoleBinding but it was not: %v", foundRoleBinding)
	}

	if foundClusterRoleBinding.Subjects[0].Name != "foo" {
		t.Errorf("expected subject to be added to ClusterRoleBinding but it was not: %v", foundClusterRoleBinding)
	}

	sandbox.Spec.Owners = []string{"foo", "bar"}

	if err := r.client.Update(ctx, &sandbox); err != nil {
		t.Fatalf("update sandbox: %v", err)
	}

	if _, err := r.Reconcile(request); err != nil {
		t.Fatalf("reconcile sandbox: %v", err)
	}

	if err := r.client.Get(ctx, types.NamespacedName{Name: roleBinding.Name, Namespace: roleBinding.Namespace}, &foundRoleBinding); err != nil {
		t.Fatalf("expected RoleBinding to exist but it does not: %v", err)
	}

	if err := r.client.Get(ctx, types.NamespacedName{Name: clusterRoleBinding.Name}, &foundClusterRoleBinding); err != nil {
		t.Fatalf("expected ClusterRoleBinding to exist but it does not: %v", err)
	}

	if foundRoleBinding.Subjects[1].Name != "bar" {
		t.Errorf("expected subject to be added to RoleBinding but it was not: %v", foundRoleBinding)
	}

	if foundClusterRoleBinding.Subjects[1].Name != "bar" {
		t.Errorf("expected subject to be added to ClusterRoleBinding but it was not: %v", foundClusterRoleBinding)
	}
}
