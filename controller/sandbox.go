package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	operatorsv1alpha1 "github.com/plexsystems/sandbox-operator/apis/operators/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var _ reconcile.Reconciler = &ReconcileSandbox{}

// SubjectsClient defines a client that gets subjects
type SubjectsClient interface {
	Subjects(ctx context.Context, users []string) ([]rbacv1.Subject, error)
}

// ReconcileSandbox reconciles a Sandbox object
type ReconcileSandbox struct {
	client         client.Client
	scheme         *runtime.Scheme
	subjectsClient SubjectsClient
}

// NewReconcileSandbox creates a new reconciler for Sandbox resources
func NewReconcileSandbox(scheme *runtime.Scheme) (*ReconcileSandbox, error) {
	client, err := NewClient(scheme)
	if err != nil {
		return nil, fmt.Errorf("new client: %w", err)
	}

	subjects, err := newSubjectsClient()
	if err != nil {
		return nil, fmt.Errorf("new subjects: %w", err)
	}

	reconcileSandbox := ReconcileSandbox{
		client:         client,
		scheme:         scheme,
		subjectsClient: subjects,
	}

	return &reconcileSandbox, nil
}

// NewClient creates a new kubernetes client
func NewClient(scheme *runtime.Scheme) (client.Client, error) {
	config, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("get config: %w", err)
	}

	client, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	return client, nil
}

// Add creates a new Sandbox controller and adds it to the controller manager
func Add(mgr manager.Manager) error {
	reconcileSandbox, err := NewReconcileSandbox(mgr.GetScheme())
	if err != nil {
		return fmt.Errorf("new reconciler: %w", err)
	}

	c, err := controller.New("sandbox-controller", mgr, controller.Options{Reconciler: reconcileSandbox})
	if err != nil {
		return fmt.Errorf("new controller: %w", err)
	}

	if err := c.Watch(&source.Kind{Type: &operatorsv1alpha1.Sandbox{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return fmt.Errorf("watch Sandbox: %w", err)
	}

	return nil
}

// Reconcile syncs Sandbox changes to the cluster
func (r *ReconcileSandbox) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.Background()

	if err := r.handleReconcile(ctx, request); err != nil {
		log.Printf("reconcile Sandbox: %v\n", err)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileSandbox) handleReconcile(ctx context.Context, request reconcile.Request) error {
	var sandbox operatorsv1alpha1.Sandbox
	if err := r.client.Get(ctx, request.NamespacedName, &sandbox); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("get Sandbox: %w", err)
	}

	namespace := getNamespace(sandbox)
	_, err := ctrl.CreateOrUpdate(ctx, r.client, &namespace, func() error {
		return controllerutil.SetControllerReference(&sandbox, &namespace, r.scheme)
	})
	if err != nil {
		return fmt.Errorf("reconcile Namespace: %w", err)
	}

	resourceQuota := getResourceQuota(sandbox)
	_, err = ctrl.CreateOrUpdate(ctx, r.client, &resourceQuota, func() error {
		return controllerutil.SetControllerReference(&sandbox, &resourceQuota, r.scheme)
	})
	if err != nil {
		return fmt.Errorf("reconcile ResourceQuota: %w", err)
	}

	role := getRole(sandbox)
	_, err = ctrl.CreateOrUpdate(ctx, r.client, &role, func() error {
		return controllerutil.SetControllerReference(&sandbox, &role, r.scheme)
	})
	if err != nil {
		return fmt.Errorf("reconcile Role: %w", err)
	}

	roleBinding := getRoleBinding(sandbox)
	_, err = ctrl.CreateOrUpdate(ctx, r.client, &roleBinding, func() error {
		subjects, err := r.subjectsClient.Subjects(ctx, sandbox.Spec.Owners)
		if err != nil {
			return fmt.Errorf("get subjects: %w", err)
		}

		roleBinding.Subjects = subjects
		return controllerutil.SetControllerReference(&sandbox, &roleBinding, r.scheme)
	})
	if err != nil {
		return fmt.Errorf("reconcile RoleBinding: %w", err)
	}

	clusterRole := getClusterRole(sandbox)
	_, err = ctrl.CreateOrUpdate(ctx, r.client, &clusterRole, func() error {
		return controllerutil.SetControllerReference(&sandbox, &clusterRole, r.scheme)
	})
	if err != nil {
		return fmt.Errorf("reconcile ClusterRole: %w", err)
	}

	clusterRoleBinding := getClusterRoleBinding(sandbox)
	_, err = ctrl.CreateOrUpdate(ctx, r.client, &clusterRoleBinding, func() error {
		subjects, err := r.subjectsClient.Subjects(ctx, sandbox.Spec.Owners)
		if err != nil {
			return fmt.Errorf("get subjects: %w", err)
		}

		clusterRoleBinding.Subjects = subjects
		return controllerutil.SetControllerReference(&sandbox, &clusterRoleBinding, r.scheme)
	})
	if err != nil {
		return fmt.Errorf("reconcile ClusterRoleBinding: %w", err)
	}

	if os.Getenv("PULL_SECRET_NAME") != "" {
		secretName := os.Getenv("PULL_SECRET_NAME")
		secretData, err := getDockerSecretData(ctx, r.client, secretName)
		if err != nil {
			return fmt.Errorf("get secret data: %w", err)
		}

		secret := getDockerSecret(sandbox, secretName, secretData)
		_, err = ctrl.CreateOrUpdate(ctx, r.client, &secret, func() error {
			return controllerutil.SetControllerReference(&sandbox, &secret, r.scheme)
		})
		if err != nil {
			return fmt.Errorf("reconcile docker Secret: %w", err)
		}

		var defaultServiceAccount corev1.ServiceAccount
		if err := r.client.Get(ctx, types.NamespacedName{Name: "default", Namespace: namespace.Name}, &defaultServiceAccount); err != nil {
			return fmt.Errorf("get default service account: %w", err)
		}

		patchBytes, err := getPatchBytes(secretName)
		if err != nil {
			return fmt.Errorf("get patch bytes: %w", err)
		}

		patch := client.ConstantPatch(types.StrategicMergePatchType, patchBytes)
		if err := r.client.Patch(ctx, &defaultServiceAccount, patch, &client.PatchOptions{}); err != nil {
			return fmt.Errorf("patch service account: %w", err)
		}
	}

	return nil
}

func getPatchBytes(secretName string) ([]byte, error) {
	type imagePullSecretsPatch struct {
		ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	}

	patch := imagePullSecretsPatch{
		ImagePullSecrets: []corev1.LocalObjectReference{
			{Name: secretName},
		},
	}

	patchString, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("patching default service account: %w", err)
	}

	return patchString, nil
}

func getNamespace(sandbox operatorsv1alpha1.Sandbox) corev1.Namespace {
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "sandbox-" + sandbox.Name,
			Labels: getCommonLabels(),
		},
	}

	return namespace
}

func getRole(sandbox operatorsv1alpha1.Sandbox) rbacv1.Role {
	role := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sandbox-" + sandbox.Name + "-owner",
			Namespace: "sandbox-" + sandbox.Name,
			Labels:    getCommonLabels(),
		},
		Rules: []rbacv1.PolicyRule{
			rbacv1.PolicyRule{
				Verbs:     []string{"*"},
				APIGroups: []string{""},
				Resources: []string{
					"pods",
					"pods/log",
					"pods/portforward",
					"services",
					"services/finalizers",
					"endpoints",
					"persistentvolumeclaims",
					"events",
					"configmaps",
					"replicationcontrollers",
				},
			},
			rbacv1.PolicyRule{
				Verbs: []string{"*"},
				APIGroups: []string{
					"apps",
					"extensions",
				},
				Resources: []string{
					"deployments",
					"daemonsets",
					"replicasets",
					"statefulsets",
				},
			},
			rbacv1.PolicyRule{
				Verbs:     []string{"*"},
				APIGroups: []string{"autoscaling"},
				Resources: []string{"horizontalpodautoscalers"},
			},
			rbacv1.PolicyRule{
				Verbs:     []string{"*"},
				APIGroups: []string{"batch"},
				Resources: []string{
					"jobs",
					"cronjobs",
				},
			},
			rbacv1.PolicyRule{
				Verbs: []string{
					"create",
					"list",
					"get",
				},
				APIGroups: []string{"rbac.authorization.k8s.io"},
				Resources: []string{
					"roles",
					"rolebindings",
				},
			},
			rbacv1.PolicyRule{
				Verbs:     []string{"create"},
				APIGroups: []string{""},
				Resources: []string{
					"secrets",
				},
			},
		},
	}

	return role
}

func getRoleBinding(sandbox operatorsv1alpha1.Sandbox) rbacv1.RoleBinding {
	roleBinding := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sandbox-" + sandbox.Name + "-owners",
			Namespace: "sandbox-" + sandbox.Name,
			Labels:    getCommonLabels(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "sandbox-" + sandbox.Name + "-owner",
		},
	}

	return roleBinding
}

func getClusterRole(sandbox operatorsv1alpha1.Sandbox) rbacv1.ClusterRole {
	clusterRole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "sandbox-" + sandbox.Name + "-admin",
			Labels: getCommonLabels(),
		},
		Rules: []rbacv1.PolicyRule{
			rbacv1.PolicyRule{
				Verbs:         []string{"delete"},
				APIGroups:     []string{"operators.plex.dev"},
				Resources:     []string{"sandboxes"},
				ResourceNames: []string{sandbox.Name},
			},
			rbacv1.PolicyRule{
				Verbs:         []string{"patch"},
				APIGroups:     []string{"operators.plex.dev"},
				Resources:     []string{"sandboxes"},
				ResourceNames: []string{sandbox.Name},
			},
		},
	}

	return clusterRole
}

func getClusterRoleBinding(sandbox operatorsv1alpha1.Sandbox) rbacv1.ClusterRoleBinding {
	clusterRoleBinding := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "sandbox-" + sandbox.Name + "-admins",
			Labels: getCommonLabels(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "sandbox-" + sandbox.Name + "-admin",
		},
	}

	return clusterRoleBinding
}

func getResourceQuota(sandbox operatorsv1alpha1.Sandbox) corev1.ResourceQuota {
	var resourceQuotaSpec corev1.ResourceQuotaSpec
	if sandbox.Spec.Size == "large" {
		resourceQuotaSpec = getLargeResourceQuotaSpec()
	} else {
		resourceQuotaSpec = getSmallResourceQuotaSpec()
	}

	resourceQuota := corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sandbox-" + sandbox.Name + "-resourcequota",
			Namespace: "sandbox-" + sandbox.Name,
			Labels:    getCommonLabels(),
		},
		Spec: resourceQuotaSpec,
	}

	return resourceQuota
}

func getLargeResourceQuotaSpec() corev1.ResourceQuotaSpec {
	resourceQuotaSpec := corev1.ResourceQuotaSpec{
		Hard: corev1.ResourceList{
			corev1.ResourceRequestsCPU:            resource.MustParse("1"),
			corev1.ResourceLimitsCPU:              resource.MustParse("2"),
			corev1.ResourceRequestsMemory:         resource.MustParse("2Gi"),
			corev1.ResourceLimitsMemory:           resource.MustParse("8Gi"),
			corev1.ResourceRequestsStorage:        resource.MustParse("40Gi"),
			corev1.ResourcePersistentVolumeClaims: resource.MustParse("8"),
		},
	}

	return resourceQuotaSpec
}

func getSmallResourceQuotaSpec() corev1.ResourceQuotaSpec {
	resourceQuotaSpec := corev1.ResourceQuotaSpec{
		Hard: corev1.ResourceList{
			corev1.ResourceRequestsCPU:            resource.MustParse("0.25"),
			corev1.ResourceLimitsCPU:              resource.MustParse("0.5"),
			corev1.ResourceRequestsMemory:         resource.MustParse("250Mi"),
			corev1.ResourceLimitsMemory:           resource.MustParse("500Mi"),
			corev1.ResourceRequestsStorage:        resource.MustParse("10Gi"),
			corev1.ResourcePersistentVolumeClaims: resource.MustParse("2"),
		},
	}

	return resourceQuotaSpec
}

func getDockerSecretData(ctx context.Context, client client.Client, secretName string) ([]byte, error) {
	var secretNamespace string
	if os.Getenv("PULL_SECRET_NAMESPACE") != "" {
		secretNamespace = os.Getenv("PULL_SECRET_NAMESPACE")
	} else {
		secretNamespace = "default"
	}

	var dockerSecret corev1.Secret
	if err := client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: secretNamespace}, &dockerSecret); err != nil {
		return nil, fmt.Errorf("get docker secret: %w", err)
	}

	if _, ok := dockerSecret.Data[corev1.DockerConfigJsonKey]; !ok {
		return nil, fmt.Errorf("secret missing dockerconfig data")
	}

	return dockerSecret.Data[corev1.DockerConfigJsonKey], nil
}

func getDockerSecret(sandbox operatorsv1alpha1.Sandbox, secretName string, secretData []byte) corev1.Secret {
	dockerSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "sandbox-" + sandbox.Name,
		},
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: []byte(secretData),
		},
		Type: corev1.SecretTypeDockerConfigJson,
	}

	return dockerSecret
}

func getCommonLabels() map[string]string {
	commonLabels := make(map[string]string)
	commonLabels["app.kubernetes.io/name"] = "sandbox-operator"
	commonLabels["app.kubernetes.io/part-of"] = "sandbox-operator"

	return commonLabels
}

// DefaultSubjects represents default subjects
type DefaultSubjects struct{}

// Subjects returns the default subjects from a given list of users
func (DefaultSubjects) Subjects(ctx context.Context, users []string) ([]rbacv1.Subject, error) {
	return getSubjects(users), nil
}

func getSubjects(users []string) []rbacv1.Subject {
	var subjects []rbacv1.Subject
	for _, user := range users {
		subject := rbacv1.Subject{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "User",
			Name:     user,
		}

		subjects = append(subjects, subject)
	}

	return subjects
}

func newSubjectsClient() (SubjectsClient, error) {
	if os.Getenv("AZURE_TENANT_ID") == "" {
		return DefaultSubjects{}, nil
	}

	azureSubjects, err := NewAzureSubjectsClient()
	if err != nil {
		return nil, fmt.Errorf("new azure subjects: %w", err)
	}

	return azureSubjects, nil
}
