package dryrun

import (
	"context"
	"path/filepath"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	helmrelease "helm.sh/helm/v3/pkg/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// TestUpgradeOpenEBSAddon tests upgrading only the OpenEBS addon in dryrun mode
func TestUpgradeOpenEBSAddon(t *testing.T) {
	// Initialize dryrun client with mocks
	kubeUtils := &dryrun.KubeUtils{}
	hcli := &helm.MockClient{}

	drFile := filepath.Join(t.TempDir(), "ec-dryrun.yaml")
	dryrun.Init(drFile, &dryrun.Client{
		KubeUtils:  kubeUtils,
		HelmClient: hcli,
	})

	// Get kube client from dryrun
	kcli, err := kubeUtils.KubeClient()
	require.NoError(t, err)

	// Create installation object in the cluster
	installation := &ecv1beta1.Installation{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Installation",
			APIVersion: "v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-installation",
		},
		Spec: ecv1beta1.InstallationSpec{
			Config: &ecv1beta1.ConfigSpec{
				Version: "v1.30.14+k0s.0",
			},
			RuntimeConfig: &ecv1beta1.RuntimeConfigSpec{
				DataDir: "/var/lib/embedded-cluster",
			},
			ClusterID: "test-cluster-id",
		},
	}

	err = kcli.Create(context.Background(), installation, &ctrlclient.CreateOptions{})
	require.NoError(t, err)

	// Mock ReleaseExists for OpenEBS (simulating it's already installed)
	hcli.On("ReleaseExists", mock.Anything, "openebs", "openebs").Return(true, nil)

	// Mock helm upgrade operation for OpenEBS
	hcli.On("Upgrade", mock.Anything, mock.MatchedBy(func(opts helm.UpgradeOptions) bool {
		return opts.ReleaseName == "openebs" && opts.Namespace == "openebs"
	})).Return(&helmrelease.Release{Name: "openebs"}, nil).Once()

	// Create runtime config and OpenEBS addon
	rc := runtimeconfig.New(installation.Spec.RuntimeConfig)
	openebsAddon := &openebs.OpenEBS{
		OpenEBSDataDir: rc.EmbeddedClusterOpenEBSLocalSubDir(),
	}

	// Call OpenEBS upgrade directly
	logf := func(format string, args ...interface{}) {
		t.Logf(format, args...)
	}

	err = openebsAddon.Upgrade(t.Context(), logf, kcli, nil, hcli, ecv1beta1.Domains{}, nil)
	require.NoError(t, err)

	// Verify helm operations were called
	hcli.AssertExpectations(t)

	// Verify helm upgrade was called with correct options
	assert.Equal(t, "Upgrade", hcli.Calls[1].Method)
	upgradeOpts := hcli.Calls[1].Arguments[1].(helm.UpgradeOptions)
	assert.Equal(t, "openebs", upgradeOpts.ReleaseName)
	assert.Equal(t, "openebs", upgradeOpts.Namespace)

	// Verify helm values for OpenEBS
	assertHelmValues(t, upgradeOpts.Values, map[string]interface{}{
		"['localpv-provisioner'].localpv.basePath": "/var/lib/embedded-cluster/openebs-local",
	})

	t.Logf("OpenEBS addon upgrade test complete")
}
