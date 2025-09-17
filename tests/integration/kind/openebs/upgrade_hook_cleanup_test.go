package openebs

import (
	"context"
	"testing"
	"time"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	"github.com/replicatedhq/embedded-cluster/tests/integration/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestOpenEBS_UpgradeHookCleanup(t *testing.T) {
	util.SetupCtrlLogging(t)

	clusterName := util.GenerateClusterName(t)
	kubeconfig := util.SetupKindCluster(t, clusterName, nil)

	kcli := util.CtrlClient(t, kubeconfig)
	mcli := util.MetadataClient(t, kubeconfig)
	hcli := util.HelmClient(t, kubeconfig)

	domains := ecv1beta1.Domains{
		ProxyRegistryDomain: "proxy.replicated.com",
	}

	addon := &openebs.OpenEBS{}

	t.Log("installing openebs")
	err := addon.Install(context.Background(), t.Logf, kcli, mcli, hcli, domains, nil)
	require.NoError(t, err, "failed to install openebs")

	t.Log("creating pre-upgrade hook resources that simulate failed helm hooks")
	hookResources := createPreUpgradeHookResources(t, kcli, addon)

	t.Log("verifying hook resources exist before upgrade")
	verifyHookResourcesExist(t, kcli, hookResources)

	t.Log("performing openebs upgrade")
	err = addon.Upgrade(context.Background(), t.Logf, kcli, mcli, hcli, domains, nil)
	require.NoError(t, err, "failed to upgrade openebs")

	// Validate that the helm upgrade occurred (a new release should be created)
	err = kcli.Get(context.Background(), client.ObjectKey{Namespace: addon.Namespace(), Name: "sh.helm.release.v1.openebs.v2"}, &corev1.Secret{})
	require.NoError(t, err, "failed to get secret")

	t.Log("verifying hook resources are cleaned up after upgrade")
	verifyHookResourcesDeleted(t, kcli, hookResources)
}

func createPreUpgradeHookResources(t *testing.T, kcli client.Client, addon *openebs.OpenEBS) []client.Object {
	namespace := addon.Namespace()
	releaseName := addon.ReleaseName()
	hookName := releaseName + "-pre-upgrade-hook"

	// Create ServiceAccount
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hookName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "openebs",
				"app.kubernetes.io/instance":  releaseName,
				"app.kubernetes.io/component": "pre-upgrade-hook",
			},
		},
	}
	err := kcli.Create(context.Background(), sa)
	require.NoError(t, err, "failed to create service account")

	// Create ClusterRole
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: hookName,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "openebs",
				"app.kubernetes.io/instance":  releaseName,
				"app.kubernetes.io/component": "pre-upgrade-hook",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumes"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
	err = kcli.Create(context.Background(), cr)
	require.NoError(t, err, "failed to create cluster role")

	// Create ClusterRoleBinding
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: hookName,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "openebs",
				"app.kubernetes.io/instance":  releaseName,
				"app.kubernetes.io/component": "pre-upgrade-hook",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     hookName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      hookName,
				Namespace: namespace,
			},
		},
	}
	err = kcli.Create(context.Background(), crb)
	require.NoError(t, err, "failed to create cluster role binding")

	// Create Job (simulating a failed helm hook)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hookName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "openebs",
				"app.kubernetes.io/instance":  releaseName,
				"app.kubernetes.io/component": "pre-upgrade-hook",
				"helm.sh/hook":                "pre-upgrade",
				"helm.sh/hook-weight":         "1",
			},
			Annotations: map[string]string{
				"helm.sh/hook-delete-policy": "hook-succeeded",
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":      "openebs",
						"app.kubernetes.io/instance":  releaseName,
						"app.kubernetes.io/component": "pre-upgrade-hook",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: hookName,
					RestartPolicy:      corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "pre-upgrade-hook",
							Image:   "busybox:1.35",
							Command: []string{"sh", "-c", "echo 'Pre-upgrade hook completed' && sleep 5"},
						},
					},
				},
			},
		},
	}
	err = kcli.Create(context.Background(), job)
	require.NoError(t, err, "failed to create job")

	return []client.Object{sa, cr, crb, job}
}

func verifyHookResourcesExist(t *testing.T, kcli client.Client, resources []client.Object) {
	for _, obj := range resources {
		err := kcli.Get(context.Background(), client.ObjectKeyFromObject(obj), obj)
		assert.NoError(t, err, "resource should exist before upgrade: %v", obj)
	}
}

func verifyHookResourcesDeleted(t *testing.T, kcli client.Client, resources []client.Object) {
	// Wait a bit for the deletion to complete
	time.Sleep(2 * time.Second)

	for _, obj := range resources {
		err := kcli.Get(context.Background(), client.ObjectKeyFromObject(obj), obj)
		assert.True(t, apierrors.IsNotFound(err), "resource should be deleted after upgrade: %v", obj)
	}
}
