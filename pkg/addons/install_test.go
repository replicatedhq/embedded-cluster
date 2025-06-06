package addons

import (
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/velero"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_getAddOnsForInstall(t *testing.T) {
	tests := []struct {
		name   string
		opts   types.InstallOptions
		before func()
		verify func(t *testing.T, addons []types.AddOn)
		after  func()
	}{
		{
			name: "online installation",
			opts: types.InstallOptions{
				IsAirgap:                  false,
				IsDisasterRecoveryEnabled: false,
				AdminConsolePassword:      "password123",
			},
			verify: func(t *testing.T, addons []types.AddOn) {
				assert.Len(t, addons, 3)

				_, ok := addons[0].(*openebs.OpenEBS)
				require.True(t, ok, "first addon should be OpenEBS")

				eco, ok := addons[1].(*embeddedclusteroperator.EmbeddedClusterOperator)
				require.True(t, ok, "second addon should be EmbeddedClusterOperator")
				assert.Empty(t, eco.ChartLocationOverride, "ECO should not have a chart location override")
				assert.Empty(t, eco.ChartVersionOverride, "ECO should not have a chart version override")
				assert.Empty(t, eco.ImageRepoOverride, "ECO should not have an image repo override")
				assert.Empty(t, eco.ImageTagOverride, "ECO should not have an image tag override")
				assert.Empty(t, eco.UtilsImageOverride, "ECO should not have a utils image override")

				_, ok = addons[2].(*adminconsole.AdminConsole)
				require.True(t, ok, "third addon should be AdminConsole")
			},
		},
		{
			name: "online installation with default domains",
			opts: types.InstallOptions{
				IsAirgap:                  false,
				IsDisasterRecoveryEnabled: false,
				AdminConsolePassword:      "password123",
			},
			before: func() {
				err := release.SetReleaseDataForTests(map[string][]byte{
					"release.yaml": []byte(`
# channel release object
defaultDomains:
  replicatedAppDomain: "staging.replicated.app"
  proxyRegistryDomain: "proxy.staging.replicated.com"
  replicatedRegistryDomain: "registry.staging.replicated.com"
`),
				})
				require.NoError(t, err)
			},
			verify: func(t *testing.T, addons []types.AddOn) {
				assert.Len(t, addons, 3)

				_, ok := addons[0].(*openebs.OpenEBS)
				require.True(t, ok, "first addon should be OpenEBS")

				_, ok = addons[1].(*embeddedclusteroperator.EmbeddedClusterOperator)
				require.True(t, ok, "second addon should be EmbeddedClusterOperator")

				_, ok = addons[2].(*adminconsole.AdminConsole)
				require.True(t, ok, "third addon should be AdminConsole")
			},
			after: func() {
				release.SetReleaseDataForTests(nil)
			},
		},
		{
			name: "online installation with custom domains",
			opts: types.InstallOptions{
				IsAirgap:                  false,
				IsDisasterRecoveryEnabled: false,
				AdminConsolePassword:      "password123",
				EmbeddedConfigSpec: &ecv1beta1.ConfigSpec{
					Domains: ecv1beta1.Domains{
						ReplicatedAppDomain:      "app.example.com",
						ProxyRegistryDomain:      "proxy.example.com",
						ReplicatedRegistryDomain: "registry.example.com",
					},
				},
			},
			before: func() {
				err := release.SetReleaseDataForTests(map[string][]byte{
					"release.yaml": []byte(`
# channel release object
defaultDomains:
  replicatedAppDomain: "staging.replicated.app"
  proxyRegistryDomain: "proxy.staging.replicated.com"
  replicatedRegistryDomain: "registry.staging.replicated.com"
`),
				})
				require.NoError(t, err)
			},
			verify: func(t *testing.T, addons []types.AddOn) {
				assert.Len(t, addons, 3)

				_, ok := addons[0].(*openebs.OpenEBS)
				require.True(t, ok, "first addon should be OpenEBS")

				_, ok = addons[1].(*embeddedclusteroperator.EmbeddedClusterOperator)
				require.True(t, ok, "second addon should be EmbeddedClusterOperator")

				_, ok = addons[2].(*adminconsole.AdminConsole)
				require.True(t, ok, "third addon should be AdminConsole")
			},
			after: func() {
				release.SetReleaseDataForTests(nil)
			},
		},
		{
			name: "airgap installation",
			opts: types.InstallOptions{
				IsAirgap:                  true,
				IsDisasterRecoveryEnabled: false,
				ServiceCIDR:               "10.96.0.0/12",
				AdminConsolePassword:      "password123",
			},
			verify: func(t *testing.T, addons []types.AddOn) {
				assert.Len(t, addons, 4)

				_, ok := addons[0].(*openebs.OpenEBS)
				require.True(t, ok, "first addon should be OpenEBS")

				eco, ok := addons[1].(*embeddedclusteroperator.EmbeddedClusterOperator)
				require.True(t, ok, "second addon should be EmbeddedClusterOperator")
				assert.Empty(t, eco.ChartLocationOverride, "ECO should not have a chart location override")
				assert.Empty(t, eco.ChartVersionOverride, "ECO should not have a chart version override")
				assert.Empty(t, eco.ImageRepoOverride, "ECO should not have an image repo override")
				assert.Empty(t, eco.ImageTagOverride, "ECO should not have an image tag override")
				assert.Empty(t, eco.UtilsImageOverride, "ECO should not have a utils image override")

				_, ok = addons[2].(*registry.Registry)
				require.True(t, ok, "third addon should be Registry")

				_, ok = addons[3].(*adminconsole.AdminConsole)
				require.True(t, ok, "fourth addon should be AdminConsole")
			},
		},
		{
			name: "disaster recovery enabled",
			opts: types.InstallOptions{
				IsAirgap:                  false,
				IsDisasterRecoveryEnabled: true,
				AdminConsolePassword:      "password123",
				ServiceCIDR:               "10.96.0.0/12",
			},
			verify: func(t *testing.T, addons []types.AddOn) {
				assert.Len(t, addons, 4)

				_, ok := addons[0].(*openebs.OpenEBS)
				require.True(t, ok, "first addon should be OpenEBS")

				eco, ok := addons[1].(*embeddedclusteroperator.EmbeddedClusterOperator)
				require.True(t, ok, "second addon should be EmbeddedClusterOperator")
				assert.Empty(t, eco.ChartLocationOverride, "ECO should not have a chart location override")
				assert.Empty(t, eco.ChartVersionOverride, "ECO should not have a chart version override")
				assert.Empty(t, eco.ImageRepoOverride, "ECO should not have an image repo override")
				assert.Empty(t, eco.ImageTagOverride, "ECO should not have an image tag override")
				assert.Empty(t, eco.UtilsImageOverride, "ECO should not have a utils image override")

				_, ok = addons[2].(*velero.Velero)
				require.True(t, ok, "third addon should be Velero")

				_, ok = addons[3].(*adminconsole.AdminConsole)
				require.True(t, ok, "fourth addon should be AdminConsole")
			},
		},
		{
			name: "airgap with disaster recovery and proxy",
			opts: types.InstallOptions{
				IsAirgap:                  true,
				IsDisasterRecoveryEnabled: true,
				ServiceCIDR:               "10.96.0.0/12",
				Proxy: &ecv1beta1.ProxySpec{
					HTTPProxy:  "http://proxy.example.com",
					HTTPSProxy: "https://proxy.example.com",
					NoProxy:    "localhost,127.0.0.1",
				},
				AdminConsolePassword: "password123",
			},
			verify: func(t *testing.T, addons []types.AddOn) {
				assert.Len(t, addons, 5)

				_, ok := addons[0].(*openebs.OpenEBS)
				require.True(t, ok, "first addon should be OpenEBS")

				eco, ok := addons[1].(*embeddedclusteroperator.EmbeddedClusterOperator)
				require.True(t, ok, "second addon should be EmbeddedClusterOperator")
				assert.Empty(t, eco.ChartLocationOverride, "ECO should not have a chart location override")
				assert.Empty(t, eco.ChartVersionOverride, "ECO should not have a chart version override")
				assert.Empty(t, eco.ImageRepoOverride, "ECO should not have an image repo override")
				assert.Empty(t, eco.ImageTagOverride, "ECO should not have an image tag override")
				assert.Empty(t, eco.UtilsImageOverride, "ECO should not have a utils image override")

				_, ok = addons[2].(*registry.Registry)
				require.True(t, ok, "third addon should be Registry")

				_, ok = addons[3].(*velero.Velero)
				require.True(t, ok, "fourth addon should be Velero")

				_, ok = addons[4].(*adminconsole.AdminConsole)
				require.True(t, ok, "fifth addon should be AdminConsole")
			},
		},
		{
			name: "airgap with disaster recovery and custom domains",
			opts: types.InstallOptions{
				IsAirgap:                  true,
				IsDisasterRecoveryEnabled: true,
				ServiceCIDR:               "10.96.0.0/12",
				AdminConsolePassword:      "password123",
				EmbeddedConfigSpec: &ecv1beta1.ConfigSpec{
					Domains: ecv1beta1.Domains{
						ReplicatedAppDomain:      "app.example.com",
						ProxyRegistryDomain:      "proxy.example.com",
						ReplicatedRegistryDomain: "registry.example.com",
					},
				},
			},
			verify: func(t *testing.T, addons []types.AddOn) {
				assert.Len(t, addons, 5)

				_, ok := addons[0].(*openebs.OpenEBS)
				require.True(t, ok, "first addon should be OpenEBS")

				eco, ok := addons[1].(*embeddedclusteroperator.EmbeddedClusterOperator)
				require.True(t, ok, "second addon should be EmbeddedClusterOperator")
				assert.Empty(t, eco.ChartLocationOverride, "ECO should not have a chart location override")
				assert.Empty(t, eco.ChartVersionOverride, "ECO should not have a chart version override")
				assert.Empty(t, eco.ImageRepoOverride, "ECO should not have an image repo override")
				assert.Empty(t, eco.ImageTagOverride, "ECO should not have an image tag override")
				assert.Empty(t, eco.UtilsImageOverride, "ECO should not have a utils image override")

				_, ok = addons[2].(*registry.Registry)
				require.True(t, ok, "third addon should be Registry")

				_, ok = addons[3].(*velero.Velero)
				require.True(t, ok, "fourth addon should be Velero")

				_, ok = addons[4].(*adminconsole.AdminConsole)
				require.True(t, ok, "fifth addon should be AdminConsole")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.before != nil {
				tt.before()
			}
			rc := runtimeconfig.New(nil)
			rc.SetDataDir(t.TempDir())
			addons := getAddOnsForInstall(nil, nil, nil, nil, rc, tt.opts)
			tt.verify(t, addons)
			if tt.after != nil {
				tt.after()
			}
		})
	}
}
