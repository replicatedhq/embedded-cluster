package addons

import (
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster/kinds/types"
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

func Test_InstallOptionsFromInstallationSpec(t *testing.T) {
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
		inSpec ecv1beta1.InstallationSpec
		meta   *ectypes.ReleaseMetadata
		verify func(t *testing.T, opts types.InstallOptions)
	}{
		{
			name: "online installation",
			inSpec: ecv1beta1.InstallationSpec{
				AirGap:           false,
				HighAvailability: false,
			},
			meta: meta,
			verify: func(t *testing.T, opts types.InstallOptions) {
				assert.False(t, opts.IsAirgap, "should not be in airgap mode")
				assert.False(t, opts.IsHA, "should not be in high availability mode")
				assert.Nil(t, opts.Proxy, "should not have a proxy")
				assert.Empty(t, opts.ServiceCIDR, "should not have a service CIDR")
			},
		},
		{
			name: "airgap installation",
			inSpec: ecv1beta1.InstallationSpec{
				AirGap:           true,
				HighAvailability: false,
				Network: &ecv1beta1.NetworkSpec{
					ServiceCIDR: "10.96.0.0/12",
				},
			},
			meta: meta,
			verify: func(t *testing.T, opts types.InstallOptions) {
				assert.True(t, opts.IsAirgap, "should be in airgap mode")
				assert.False(t, opts.IsHA, "should not be in high availability mode")
				assert.Nil(t, opts.Proxy, "should not have a proxy")
				assert.Equal(t, "10.96.0.0/12", opts.ServiceCIDR, "should have a service cidr")
			},
		},
		{
			name: "with disaster recovery",
			inSpec: ecv1beta1.InstallationSpec{
				AirGap:           false,
				HighAvailability: false,
				Network: &ecv1beta1.NetworkSpec{
					ServiceCIDR: "10.96.0.0/12",
				},
				LicenseInfo: &ecv1beta1.LicenseInfo{
					IsDisasterRecoverySupported: true,
				},
			},
			meta: meta,
			verify: func(t *testing.T, opts types.InstallOptions) {
				assert.False(t, opts.IsAirgap, "should not be in airgap mode")
				assert.False(t, opts.IsHA, "should not be in high availability mode")
				assert.Nil(t, opts.Proxy, "should not have a proxy")
				assert.Equal(t, "10.96.0.0/12", opts.ServiceCIDR, "should have a service cidr")
				assert.True(t, opts.IsDisasterRecoveryEnabled, "disaster recovery should be enabled")
			},
		},
		{
			name: "airgap HA with proxy and disaster recovery",
			inSpec: ecv1beta1.InstallationSpec{
				AirGap:           true,
				HighAvailability: true,
				Network: &ecv1beta1.NetworkSpec{
					ServiceCIDR: "10.96.0.0/12",
				},
				LicenseInfo: &ecv1beta1.LicenseInfo{
					IsDisasterRecoverySupported: true,
				},
				Proxy: &ecv1beta1.ProxySpec{
					HTTPProxy:  "http://proxy.example.com",
					HTTPSProxy: "https://proxy.example.com",
					NoProxy:    "localhost,127.0.0.1",
				},
			},
			meta: meta,
			verify: func(t *testing.T, opts types.InstallOptions) {
				assert.True(t, opts.IsAirgap, "should be in airgap mode")
				assert.True(t, opts.IsHA, "should be in high availability mode")
				assert.NotNil(t, opts.Proxy, "should have a proxy")
				assert.Equal(t, "http://proxy.example.com", opts.Proxy.HTTPProxy, "http proxy should be set")
				assert.Equal(t, "https://proxy.example.com", opts.Proxy.HTTPSProxy, "https proxy should be set")
				assert.Equal(t, "localhost,127.0.0.1", opts.Proxy.NoProxy, "no proxy should be set")
				assert.Equal(t, "10.96.0.0/12", opts.ServiceCIDR, "should have a service cidr")
				assert.True(t, opts.IsDisasterRecoveryEnabled, "disaster recovery should be enabled")

			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := InstallOptionsFromInstallationSpec(tt.inSpec)
			tt.verify(t, opts)
		})
	}
}

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
