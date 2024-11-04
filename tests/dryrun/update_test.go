package dryrun

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/k0s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestUpdateAirgapNewFS(t *testing.T) {
	updateCmdSetupFilesystem(t, "/var/lib/embedded-cluster", "/var/lib/embedded-cluster/k0s")

	k0sClient := &dryrun.K0sClient{}
	kubeUtils := &dryrun.KubeUtils{}

	drFile := filepath.Join(t.TempDir(), "ec-dryrun.yaml")
	dryrun.Init(drFile, &dryrun.Client{
		KubeUtils: kubeUtils,
		K0sClient: k0sClient,
	})

	k0sClient.Status = &k0s.K0sStatus{
		Vars: k0s.K0sVars{
			AdminKubeConfigPath: "/var/lib/embedded-cluster/k0s/pki/admin.conf",
		},
	}

	kubeClient, err := kubeUtils.KubeClient()
	require.NoError(t, err)

	kubeClient.Create(context.Background(), &ecv1beta1.Installation{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Installation",
			APIVersion: "v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "20241002205018",
		},
		Spec: ecv1beta1.InstallationSpec{
			Config: &ecv1beta1.ConfigSpec{
				Version: "1.17.0+k8s-1.30",
			},
			RuntimeConfig: &ecv1beta1.RuntimeConfigSpec{
				DataDir: "/var/lib/embedded-cluster",
			},
		},
	}, &ctrlclient.CreateOptions{})

	dr := dryrunUpdate(t,
		"--airgap-bundle", "./assets/bundle.airgap",
	)

	// --- validate os env --- //
	assertEnv(t, dr.OSEnv, map[string]string{
		"TMPDIR":     "/var/lib/embedded-cluster/tmp",
		"KUBECONFIG": "/var/lib/embedded-cluster/k0s/pki/admin.conf",
	})

	// --- validate commands --- //
	found := false
	for _, c := range dr.Commands {
		if strings.Contains(c.Cmd, "airgap-update fake-app-slug") {
			assert.Contains(t, c.Cmd, "airgap-update fake-app-slug --namespace kotsadm --airgap-bundle ./assets/bundle.airgap")
			found = true
		}
	}
	require.True(t, found, "unable to find kubectl-kots airgap-update command")
}

func TestUpdateAirgapPreFS(t *testing.T) {
	updateCmdSetupFilesystem(t, "/var/lib/embedded-cluster", "/var/lib/k0s")

	k0sClient := &dryrun.K0sClient{}
	kubeUtils := &dryrun.KubeUtils{}

	drFile := filepath.Join(t.TempDir(), "ec-dryrun.yaml")
	dryrun.Init(drFile, &dryrun.Client{
		KubeUtils: kubeUtils,
		K0sClient: k0sClient,
	})

	k0sClient.Status = &k0s.K0sStatus{
		Vars: k0s.K0sVars{
			AdminKubeConfigPath: "/var/lib/k0s/pki/admin.conf",
		},
	}

	kubeClient, err := kubeUtils.KubeClient()
	require.NoError(t, err)

	kubeClient.Create(context.Background(), &ecv1beta1.Installation{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Installation",
			APIVersion: "v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "20241002205018",
		},
		Spec: ecv1beta1.InstallationSpec{
			Config: &ecv1beta1.ConfigSpec{
				Version: "1.14.2+k8s-1.29",
			},
			RuntimeConfig: &ecv1beta1.RuntimeConfigSpec{},
		},
	}, &ctrlclient.CreateOptions{})

	dr := dryrunUpdate(t,
		"--airgap-bundle", "./assets/bundle.airgap",
	)

	// --- validate os env --- //
	assertEnv(t, dr.OSEnv, map[string]string{
		"TMPDIR":     "/var/lib/embedded-cluster/tmp",
		"KUBECONFIG": "/var/lib/k0s/pki/admin.conf",
	})

	// --- validate commands --- //
	found := false
	for _, c := range dr.Commands {
		if strings.Contains(c.Cmd, "airgap-update fake-app-slug") {
			assert.Contains(t, c.Cmd, "airgap-update fake-app-slug --namespace kotsadm --airgap-bundle ./assets/bundle.airgap")
			found = true
		}
	}
	require.True(t, found, "unable to find kubectl-kots airgap-update command")
}

func updateCmdSetupFilesystem(t *testing.T, root, k0s string) {
	err := os.MkdirAll(root, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(root, "tmp"), 0755)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(k0s, "pki"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(k0s, "pki/ca.crt"), []byte("fake-ca-cert"), 0644)
	require.NoError(t, err)
}
