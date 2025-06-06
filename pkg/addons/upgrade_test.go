package addons

import (
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/velero"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_getAddOnsForUpgrade(t *testing.T) {
	meta := &ectypes.ReleaseMetadata{
		Configs: ecv1beta1.Helm{
			Charts: []ecv1beta1.Chart{
				{
					Name:      "embedded-cluster-operator",
					ChartName: "replicated/embedded-cluster-operator",
					Version:   "1.22.0+k8s-1.30",
				},
			},
		},
		Images: []string{
			"proxy.replicated.com/anonymous/replicated/embedded-cluster-operator-image:1.22.0-k8s-1.30-amd64@sha256:929b6cb42add383a69e3b26790c06320bd4eac0ecd60b509212c1864d69c6a88",
			"proxy.replicated.com/anonymous/replicated/ec-utils:latest-amd64@sha256:f499ed26bd5899bc5a1ae14d9d13853d1fc615ae21bde86fe250960772fd2c70",
		},
	}

	tests := []struct {
		name   string
		opts   types.InstallOptions
		meta   *ectypes.ReleaseMetadata
		verify func(t *testing.T, addons []types.AddOn, err error)
	}{
		{
			name: "online installation",
			opts: types.InstallOptions{
				IsAirgap: false,
				IsHA:     false,
			},
			meta: meta,
			verify: func(t *testing.T, addons []types.AddOn, err error) {
				assert.NoError(t, err)
				assert.Len(t, addons, 3)

				_, ok := addons[0].(*openebs.OpenEBS)
				require.True(t, ok, "first addon should be OpenEBS")

				eco, ok := addons[1].(*embeddedclusteroperator.EmbeddedClusterOperator)
				require.True(t, ok, "second addon should be EmbeddedClusterOperator")
				assert.Equal(t, "replicated/embedded-cluster-operator", eco.ChartLocationOverride)
				assert.Equal(t, "1.22.0+k8s-1.30", eco.ChartVersionOverride)
				assert.Equal(t, "proxy.replicated.com/anonymous/replicated/embedded-cluster-operator-image", eco.ImageRepoOverride)
				assert.Equal(t, "1.22.0-k8s-1.30-amd64@sha256:929b6cb42add383a69e3b26790c06320bd4eac0ecd60b509212c1864d69c6a88", eco.ImageTagOverride)
				assert.Equal(t, "proxy.replicated.com/anonymous/replicated/ec-utils:latest-amd64@sha256:f499ed26bd5899bc5a1ae14d9d13853d1fc615ae21bde86fe250960772fd2c70", eco.UtilsImageOverride)

				_, ok = addons[2].(*adminconsole.AdminConsole)
				require.True(t, ok, "third addon should be AdminConsole")
			},
		},
		{
			name: "airgap installation",
			opts: types.InstallOptions{
				IsAirgap:    true,
				IsHA:        false,
				ServiceCIDR: "10.96.0.0/12",
			},
			meta: meta,
			verify: func(t *testing.T, addons []types.AddOn, err error) {
				assert.NoError(t, err)
				assert.Len(t, addons, 4)

				_, ok := addons[0].(*openebs.OpenEBS)
				require.True(t, ok, "first addon should be OpenEBS")

				eco, ok := addons[1].(*embeddedclusteroperator.EmbeddedClusterOperator)
				require.True(t, ok, "second addon should be EmbeddedClusterOperator")
				assert.Equal(t, "replicated/embedded-cluster-operator", eco.ChartLocationOverride)
				assert.Equal(t, "1.22.0+k8s-1.30", eco.ChartVersionOverride)
				assert.Equal(t, "proxy.replicated.com/anonymous/replicated/embedded-cluster-operator-image", eco.ImageRepoOverride)
				assert.Equal(t, "1.22.0-k8s-1.30-amd64@sha256:929b6cb42add383a69e3b26790c06320bd4eac0ecd60b509212c1864d69c6a88", eco.ImageTagOverride)
				assert.Equal(t, "proxy.replicated.com/anonymous/replicated/ec-utils:latest-amd64@sha256:f499ed26bd5899bc5a1ae14d9d13853d1fc615ae21bde86fe250960772fd2c70", eco.UtilsImageOverride)

				_, ok = addons[2].(*registry.Registry)
				require.True(t, ok, "third addon should be Registry")

				_, ok = addons[3].(*adminconsole.AdminConsole)
				require.True(t, ok, "fourth addon should be AdminConsole")
			},
		},
		{
			name: "with disaster recovery",
			opts: types.InstallOptions{
				IsAirgap:                  false,
				IsHA:                      false,
				ServiceCIDR:               "10.96.0.0/12",
				IsDisasterRecoveryEnabled: true,
			},
			meta: meta,
			verify: func(t *testing.T, addons []types.AddOn, err error) {
				assert.NoError(t, err)
				assert.Len(t, addons, 4)

				_, ok := addons[0].(*openebs.OpenEBS)
				require.True(t, ok, "first addon should be OpenEBS")

				eco, ok := addons[1].(*embeddedclusteroperator.EmbeddedClusterOperator)
				require.True(t, ok, "second addon should be EmbeddedClusterOperator")
				assert.Equal(t, "replicated/embedded-cluster-operator", eco.ChartLocationOverride)
				assert.Equal(t, "1.22.0+k8s-1.30", eco.ChartVersionOverride)
				assert.Equal(t, "proxy.replicated.com/anonymous/replicated/embedded-cluster-operator-image", eco.ImageRepoOverride)
				assert.Equal(t, "1.22.0-k8s-1.30-amd64@sha256:929b6cb42add383a69e3b26790c06320bd4eac0ecd60b509212c1864d69c6a88", eco.ImageTagOverride)
				assert.Equal(t, "proxy.replicated.com/anonymous/replicated/ec-utils:latest-amd64@sha256:f499ed26bd5899bc5a1ae14d9d13853d1fc615ae21bde86fe250960772fd2c70", eco.UtilsImageOverride)

				_, ok = addons[2].(*velero.Velero)
				require.True(t, ok, "third addon should be Velero")

				_, ok = addons[3].(*adminconsole.AdminConsole)
				require.True(t, ok, "fourth addon should be AdminConsole")
			},
		},
		{
			name: "airgap HA with proxy and disaster recovery",
			opts: types.InstallOptions{
				IsAirgap:                  true,
				IsHA:                      true,
				ServiceCIDR:               "10.96.0.0/12",
				IsDisasterRecoveryEnabled: true,
				Proxy: &ecv1beta1.ProxySpec{
					HTTPProxy:  "http://proxy.example.com",
					HTTPSProxy: "https://proxy.example.com",
					NoProxy:    "localhost,127.0.0.1",
				},
			},
			meta: meta,
			verify: func(t *testing.T, addons []types.AddOn, err error) {
				assert.NoError(t, err)
				assert.Len(t, addons, 6)

				_, ok := addons[0].(*openebs.OpenEBS)
				require.True(t, ok, "first addon should be OpenEBS")

				eco, ok := addons[1].(*embeddedclusteroperator.EmbeddedClusterOperator)
				require.True(t, ok, "second addon should be EmbeddedClusterOperator")
				assert.Equal(t, "replicated/embedded-cluster-operator", eco.ChartLocationOverride)
				assert.Equal(t, "1.22.0+k8s-1.30", eco.ChartVersionOverride)
				assert.Equal(t, "proxy.replicated.com/anonymous/replicated/embedded-cluster-operator-image", eco.ImageRepoOverride)
				assert.Equal(t, "1.22.0-k8s-1.30-amd64@sha256:929b6cb42add383a69e3b26790c06320bd4eac0ecd60b509212c1864d69c6a88", eco.ImageTagOverride)
				assert.Equal(t, "proxy.replicated.com/anonymous/replicated/ec-utils:latest-amd64@sha256:f499ed26bd5899bc5a1ae14d9d13853d1fc615ae21bde86fe250960772fd2c70", eco.UtilsImageOverride)

				_, ok = addons[2].(*registry.Registry)
				require.True(t, ok, "third addon should be Registry")

				_, ok = addons[3].(*seaweedfs.SeaweedFS)
				require.True(t, ok, "fourth addon should be SeaweedFS")

				_, ok = addons[4].(*velero.Velero)
				require.True(t, ok, "fifth addon should be Velero")

				_, ok = addons[5].(*adminconsole.AdminConsole)
				require.True(t, ok, "sixth addon should be AdminConsole")
			},
		},
		{
			name: "invalid metadata - missing chart",
			opts: types.InstallOptions{},
			meta: &ectypes.ReleaseMetadata{
				Configs: ecv1beta1.Helm{
					Charts: []ecv1beta1.Chart{},
				},
				Images: meta.Images,
			},
			verify: func(t *testing.T, addons []types.AddOn, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "no embedded-cluster-operator chart found")
			},
		},
		{
			name: "invalid metadata - missing images",
			opts: types.InstallOptions{},
			meta: &ectypes.ReleaseMetadata{
				Configs: meta.Configs,
				Images:  []string{},
			},
			verify: func(t *testing.T, addons []types.AddOn, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "no embedded-cluster-operator-image found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := runtimeconfig.New(nil)
			rc.SetDataDir(t.TempDir())
			addons, err := getAddOnsForUpgrade(nil, nil, nil, nil, rc, tt.meta, tt.opts)
			tt.verify(t, addons, err)
		})
	}
}
