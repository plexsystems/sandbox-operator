// +build integration

package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/plexsystems/sandbox-operator/apis"
	operatorsv1alpha1 "github.com/plexsystems/sandbox-operator/apis/operators/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestSandboxControllerIntegration(t *testing.T) {
	ctx := context.TODO()

	s := scheme.Scheme
	s.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.Sandbox{})
	apis.AddToScheme(s)

	client, err := NewClient(s)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	sandbox := operatorsv1alpha1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: operatorsv1alpha1.SandboxSpec{
			Owners: []string{"foo@bar.com"},
		},
	}

	if err := client.Create(ctx, &sandbox); err != nil {
		t.Fatalf("create sandbox: %v", err)
	}

	const intervalTime = 5 * time.Second
	const waitTime = 30 * time.Second

	namespace := getNamespace(sandbox)
	err = wait.PollImmediate(intervalTime, waitTime, func() (bool, error) {
		geterr := client.Get(ctx, types.NamespacedName{Name: namespace.Name}, &corev1.Namespace{})
		if geterr == nil {
			return true, nil
		} else if errors.IsNotFound(geterr) {
			return false, nil
		} else {
			return false, fmt.Errorf("get namespace: %w", geterr)
		}
	})
	if err != nil {
		t.Errorf("namespace not found: %v", err)
	}

	role := getRole(sandbox)
	err = wait.PollImmediate(intervalTime, waitTime, func() (bool, error) {
		geterr := client.Get(ctx, types.NamespacedName{Namespace: role.Namespace, Name: role.Name}, &rbacv1.Role{})
		if geterr == nil {
			return true, nil
		} else if errors.IsNotFound(geterr) {
			return false, nil
		} else {
			return false, fmt.Errorf("get role: %w", geterr)
		}
	})
	if err != nil {
		t.Errorf("role not found: %v", err)
	}

	if err := client.Delete(ctx, &sandbox); err != nil {
		t.Fatalf("delete sandbox: %v", err)
	}

	err = wait.PollImmediate(intervalTime, waitTime, func() (bool, error) {
		geterr := client.Get(ctx, types.NamespacedName{Name: namespace.Name}, &corev1.Namespace{})
		if errors.IsNotFound(geterr) {
			return true, nil
		} else if err == nil {
			return false, nil
		} else {
			return false, fmt.Errorf("get sandbox: %v", err)
		}
	})
	if err != nil {
		t.Errorf("namespace not deleted: %v", err)
	}
}
