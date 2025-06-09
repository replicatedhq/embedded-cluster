package dryrun

import (
	"context"
	_ "embed"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// TestUpdateAirgapCurrent tests the update command with a filesystem equivalent to the current
// version.
func TestUpdateAirgapCurrent(t *testing.T) {
	updateCmdSetupFilesystem(t, "/var/lib/embedded-cluster", "/var/lib/embedded-cluster/k0s")

	k0sClient := &dryrun.K0s{}
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
		"--airgap-bundle", airgapBundleFile(t),
	)

	// --- validate os env --- //
	assertEnv(t, dr.OSEnv, map[string]string{
		"TMPDIR":     "/var/lib/embedded-cluster/tmp",
		"KUBECONFIG": "/var/lib/embedded-cluster/k0s/pki/admin.conf",
	})

	// --- validate commands --- //
	assertCommands(t, dr.Commands,
		[]interface{}{
			regexp.MustCompile("airgap-update fake-app-slug --namespace kotsadm --airgap-bundle .*/bundle.airgap"),
		},
		true,
	)
}

// TestUpdateAirgapPreFS tests the update command with a filesystem equivalent to a version before
// the data directories were consolidated under /var/lib/embedded-cluster.
func TestUpdateAirgapPreFS(t *testing.T) {
	updateCmdSetupFilesystem(t, "/var/lib/embedded-cluster", "/var/lib/k0s")

	k0sClient := &dryrun.K0s{}
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
		"--airgap-bundle", airgapBundleFile(t),
	)

	// --- validate os env --- //
	assertEnv(t, dr.OSEnv, map[string]string{
		"TMPDIR":     "/var/lib/embedded-cluster/tmp",
		"KUBECONFIG": "/var/lib/k0s/pki/admin.conf",
	})

	// --- validate commands --- //
	assertCommands(t, dr.Commands,
		[]interface{}{
			regexp.MustCompile("airgap-update fake-app-slug --namespace kotsadm --airgap-bundle .*/bundle.airgap"),
		},
		true,
	)
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

var (
	//go:embed assets/bundle.airgap
	airgapBundle []byte
)

func airgapBundleFile(t *testing.T) string {
	bundleAirgapFile := filepath.Join(t.TempDir(), "bundle.airgap")
	require.NoError(t, os.WriteFile(bundleAirgapFile, airgapBundle, 0644))
	return bundleAirgapFile
}
