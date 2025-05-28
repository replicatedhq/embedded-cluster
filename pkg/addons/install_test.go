package addons

import (
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/domains"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/velero"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_getAddOnsForInstall(t *testing.T) {
	tests := []struct {
		name   string
		opts   InstallOptions
		before func()
		verify func(t *testing.T, addons []types.AddOn)
		after  func()
	}{
		{
			name: "online installation",
			opts: InstallOptions{
				IsAirgap:                false,
				DisasterRecoveryEnabled: false,
				AdminConsolePwd:         "password123",
			},
			verify: func(t *testing.T, addons []types.AddOn) {
				assert.Len(t, addons, 3)

				openEBS, ok := addons[0].(*openebs.OpenEBS)
				require.True(t, ok, "first addon should be OpenEBS")
				assert.Equal(t, domains.DefaultProxyRegistryDomain, openEBS.ProxyRegistryDomain)

				eco, ok := addons[1].(*embeddedclusteroperator.EmbeddedClusterOperator)
				require.True(t, ok, "second addon should be EmbeddedClusterOperator")
				assert.False(t, eco.IsAirgap, "ECO should not be in airgap mode")
				assert.Nil(t, eco.Proxy, "ECO should not have a proxy")
				assert.Empty(t, eco.ChartLocationOverride, "ECO should not have a chart location override")
				assert.Empty(t, eco.ChartVersionOverride, "ECO should not have a chart version override")
				assert.Empty(t, eco.ImageRepoOverride, "ECO should not have an image repo override")
				assert.Empty(t, eco.ImageTagOverride, "ECO should not have an image tag override")
				assert.Empty(t, eco.UtilsImageOverride, "ECO should not have a utils image override")
				assert.Equal(t, domains.DefaultProxyRegistryDomain, eco.ProxyRegistryDomain)

				adminConsole, ok := addons[2].(*adminconsole.AdminConsole)
				require.True(t, ok, "third addon should be AdminConsole")
				assert.False(t, adminConsole.IsAirgap, "AdminConsole should not be in airgap mode")
				assert.False(t, adminConsole.IsHA, "AdminConsole should not be in high availability mode")
				assert.Nil(t, adminConsole.Proxy, "AdminConsole should not have a proxy")
				assert.Empty(t, adminConsole.ServiceCIDR, "AdminConsole should not have a service CIDR")
				assert.Equal(t, "password123", adminConsole.Password)
				assert.Equal(t, domains.DefaultReplicatedAppDomain, adminConsole.ReplicatedAppDomain)
				assert.Equal(t, domains.DefaultProxyRegistryDomain, adminConsole.ProxyRegistryDomain)
				assert.Equal(t, domains.DefaultReplicatedRegistryDomain, adminConsole.ReplicatedRegistryDomain)
			},
		},
		{
			name: "online installation with default domains",
			opts: InstallOptions{
				IsAirgap:                false,
				DisasterRecoveryEnabled: false,
				AdminConsolePwd:         "password123",
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

				openEBS, ok := addons[0].(*openebs.OpenEBS)
				require.True(t, ok, "first addon should be OpenEBS")
				assert.Equal(t, "proxy.staging.replicated.com", openEBS.ProxyRegistryDomain)

				eco, ok := addons[1].(*embeddedclusteroperator.EmbeddedClusterOperator)
				require.True(t, ok, "second addon should be EmbeddedClusterOperator")
				assert.Equal(t, "proxy.staging.replicated.com", eco.ProxyRegistryDomain)

				adminConsole, ok := addons[2].(*adminconsole.AdminConsole)
				require.True(t, ok, "third addon should be AdminConsole")
				assert.Equal(t, "staging.replicated.app", adminConsole.ReplicatedAppDomain)
				assert.Equal(t, "proxy.staging.replicated.com", adminConsole.ProxyRegistryDomain)
				assert.Equal(t, "registry.staging.replicated.com", adminConsole.ReplicatedRegistryDomain)
			},
			after: func() {
				release.SetReleaseDataForTests(nil)
			},
		},
		{
			name: "online installation with custom domains",
			opts: InstallOptions{
				IsAirgap:                false,
				DisasterRecoveryEnabled: false,
				AdminConsolePwd:         "password123",
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

				openEBS, ok := addons[0].(*openebs.OpenEBS)
				require.True(t, ok, "first addon should be OpenEBS")
				assert.Equal(t, "proxy.example.com", openEBS.ProxyRegistryDomain)

				eco, ok := addons[1].(*embeddedclusteroperator.EmbeddedClusterOperator)
				require.True(t, ok, "second addon should be EmbeddedClusterOperator")
				assert.Equal(t, "proxy.example.com", eco.ProxyRegistryDomain)

				adminConsole, ok := addons[2].(*adminconsole.AdminConsole)
				require.True(t, ok, "third addon should be AdminConsole")
				assert.Equal(t, "app.example.com", adminConsole.ReplicatedAppDomain)
				assert.Equal(t, "proxy.example.com", adminConsole.ProxyRegistryDomain)
				assert.Equal(t, "registry.example.com", adminConsole.ReplicatedRegistryDomain)
			},
			after: func() {
				release.SetReleaseDataForTests(nil)
			},
		},
		{
			name: "airgap installation",
			opts: InstallOptions{
				IsAirgap:                true,
				DisasterRecoveryEnabled: false,
				ServiceCIDR:             "10.96.0.0/12",
				AdminConsolePwd:         "password123",
			},
			verify: func(t *testing.T, addons []types.AddOn) {
				assert.Len(t, addons, 4)

				openEBS, ok := addons[0].(*openebs.OpenEBS)
				require.True(t, ok, "first addon should be OpenEBS")
				assert.Equal(t, domains.DefaultProxyRegistryDomain, openEBS.ProxyRegistryDomain)

				eco, ok := addons[1].(*embeddedclusteroperator.EmbeddedClusterOperator)
				require.True(t, ok, "second addon should be EmbeddedClusterOperator")
				assert.True(t, eco.IsAirgap, "ECO should be in airgap mode")
				assert.Nil(t, eco.Proxy, "ECO should not have a proxy")
				assert.Empty(t, eco.ChartLocationOverride, "ECO should not have a chart location override")
				assert.Empty(t, eco.ChartVersionOverride, "ECO should not have a chart version override")
				assert.Empty(t, eco.ImageRepoOverride, "ECO should not have an image repo override")
				assert.Empty(t, eco.ImageTagOverride, "ECO should not have an image tag override")
				assert.Empty(t, eco.UtilsImageOverride, "ECO should not have a utils image override")
				assert.Equal(t, domains.DefaultProxyRegistryDomain, eco.ProxyRegistryDomain)

				reg, ok := addons[2].(*registry.Registry)
				require.True(t, ok, "third addon should be Registry")
				assert.Equal(t, "10.96.0.0/12", reg.ServiceCIDR)
				assert.Equal(t, domains.DefaultProxyRegistryDomain, reg.ProxyRegistryDomain)

				adminConsole, ok := addons[3].(*adminconsole.AdminConsole)
				require.True(t, ok, "fourth addon should be AdminConsole")
				assert.True(t, adminConsole.IsAirgap, "AdminConsole should be in airgap mode")
				assert.False(t, adminConsole.IsHA, "AdminConsole should not be in high availability mode")
				assert.Nil(t, adminConsole.Proxy, "AdminConsole should not have a proxy")
				assert.Equal(t, "10.96.0.0/12", adminConsole.ServiceCIDR)
				assert.Equal(t, "password123", adminConsole.Password)
				assert.Equal(t, domains.DefaultReplicatedAppDomain, adminConsole.ReplicatedAppDomain)
				assert.Equal(t, domains.DefaultProxyRegistryDomain, adminConsole.ProxyRegistryDomain)
				assert.Equal(t, domains.DefaultReplicatedRegistryDomain, adminConsole.ReplicatedRegistryDomain)
			},
		},
		{
			name: "disaster recovery enabled",
			opts: InstallOptions{
				IsAirgap:                false,
				DisasterRecoveryEnabled: true,
				AdminConsolePwd:         "password123",
				ServiceCIDR:             "10.96.0.0/12",
			},
			verify: func(t *testing.T, addons []types.AddOn) {
				assert.Len(t, addons, 4)

				openEBS, ok := addons[0].(*openebs.OpenEBS)
				require.True(t, ok, "first addon should be OpenEBS")
				assert.Equal(t, domains.DefaultProxyRegistryDomain, openEBS.ProxyRegistryDomain)

				eco, ok := addons[1].(*embeddedclusteroperator.EmbeddedClusterOperator)
				require.True(t, ok, "second addon should be EmbeddedClusterOperator")
				assert.False(t, eco.IsAirgap, "ECO should not be in airgap mode")
				assert.Nil(t, eco.Proxy, "ECO should not have a proxy")
				assert.Empty(t, eco.ChartLocationOverride, "ECO should not have a chart location override")
				assert.Empty(t, eco.ChartVersionOverride, "ECO should not have a chart version override")
				assert.Empty(t, eco.ImageRepoOverride, "ECO should not have an image repo override")
				assert.Empty(t, eco.ImageTagOverride, "ECO should not have an image tag override")
				assert.Empty(t, eco.UtilsImageOverride, "ECO should not have a utils image override")
				assert.Equal(t, domains.DefaultProxyRegistryDomain, eco.ProxyRegistryDomain)

				vel, ok := addons[2].(*velero.Velero)
				require.True(t, ok, "third addon should be Velero")
				assert.Nil(t, vel.Proxy, "Velero should not have a proxy")
				assert.Equal(t, domains.DefaultProxyRegistryDomain, vel.ProxyRegistryDomain)

				adminConsole, ok := addons[3].(*adminconsole.AdminConsole)
				require.True(t, ok, "fourth addon should be AdminConsole")
				assert.False(t, eco.IsAirgap, "AdminConsole should not be in airgap mode")
				assert.False(t, adminConsole.IsHA, "AdminConsole should not be in high availability mode")
				assert.Nil(t, adminConsole.Proxy, "AdminConsole should not have a proxy")
				assert.Equal(t, "10.96.0.0/12", adminConsole.ServiceCIDR)
				assert.Equal(t, "password123", adminConsole.Password)
				assert.Equal(t, domains.DefaultReplicatedAppDomain, adminConsole.ReplicatedAppDomain)
				assert.Equal(t, domains.DefaultProxyRegistryDomain, adminConsole.ProxyRegistryDomain)
				assert.Equal(t, domains.DefaultReplicatedRegistryDomain, adminConsole.ReplicatedRegistryDomain)
			},
		},
		{
			name: "airgap with disaster recovery and proxy",
			opts: InstallOptions{
				IsAirgap:                true,
				DisasterRecoveryEnabled: true,
				ServiceCIDR:             "10.96.0.0/12",
				Proxy: &ecv1beta1.ProxySpec{
					HTTPProxy:  "http://proxy.example.com",
					HTTPSProxy: "https://proxy.example.com",
					NoProxy:    "localhost,127.0.0.1",
				},
				AdminConsolePwd: "password123",
			},
			verify: func(t *testing.T, addons []types.AddOn) {
				assert.Len(t, addons, 5)

				openEBS, ok := addons[0].(*openebs.OpenEBS)
				require.True(t, ok, "first addon should be OpenEBS")
				assert.Equal(t, domains.DefaultProxyRegistryDomain, openEBS.ProxyRegistryDomain)

				eco, ok := addons[1].(*embeddedclusteroperator.EmbeddedClusterOperator)
				require.True(t, ok, "second addon should be EmbeddedClusterOperator")
				assert.True(t, eco.IsAirgap, "ECO should be in airgap mode")
				assert.Equal(t, "http://proxy.example.com", eco.Proxy.HTTPProxy)
				assert.Equal(t, "https://proxy.example.com", eco.Proxy.HTTPSProxy)
				assert.Equal(t, "localhost,127.0.0.1", eco.Proxy.NoProxy)
				assert.Empty(t, eco.ChartLocationOverride, "ECO should not have a chart location override")
				assert.Empty(t, eco.ChartVersionOverride, "ECO should not have a chart version override")
				assert.Empty(t, eco.ImageRepoOverride, "ECO should not have an image repo override")
				assert.Empty(t, eco.ImageTagOverride, "ECO should not have an image tag override")
				assert.Empty(t, eco.UtilsImageOverride, "ECO should not have a utils image override")
				assert.Equal(t, domains.DefaultProxyRegistryDomain, eco.ProxyRegistryDomain)

				reg, ok := addons[2].(*registry.Registry)
				require.True(t, ok, "third addon should be Registry")
				assert.Equal(t, "10.96.0.0/12", reg.ServiceCIDR)
				assert.False(t, reg.IsHA, "Registry should not be in high availability mode")
				assert.Equal(t, domains.DefaultProxyRegistryDomain, reg.ProxyRegistryDomain)

				vel, ok := addons[3].(*velero.Velero)
				require.True(t, ok, "fourth addon should be Velero")
				assert.Equal(t, "http://proxy.example.com", vel.Proxy.HTTPProxy)
				assert.Equal(t, "https://proxy.example.com", vel.Proxy.HTTPSProxy)
				assert.Equal(t, "localhost,127.0.0.1", vel.Proxy.NoProxy)
				assert.Equal(t, domains.DefaultProxyRegistryDomain, vel.ProxyRegistryDomain)

				adminConsole, ok := addons[4].(*adminconsole.AdminConsole)
				require.True(t, ok, "fifth addon should be AdminConsole")
				assert.True(t, adminConsole.IsAirgap, "AdminConsole should be in airgap mode")
				assert.False(t, adminConsole.IsHA, "AdminConsole should not be in high availability mode")
				assert.Equal(t, "http://proxy.example.com", adminConsole.Proxy.HTTPProxy)
				assert.Equal(t, "https://proxy.example.com", adminConsole.Proxy.HTTPSProxy)
				assert.Equal(t, "localhost,127.0.0.1", adminConsole.Proxy.NoProxy)
				assert.Equal(t, "10.96.0.0/12", adminConsole.ServiceCIDR)
				assert.Equal(t, "password123", adminConsole.Password)
				assert.Equal(t, domains.DefaultReplicatedAppDomain, adminConsole.ReplicatedAppDomain)
				assert.Equal(t, domains.DefaultProxyRegistryDomain, adminConsole.ProxyRegistryDomain)
				assert.Equal(t, domains.DefaultReplicatedRegistryDomain, adminConsole.ReplicatedRegistryDomain)
			},
		},
		{
			name: "airgap with disaster recovery and custom domains",
			opts: InstallOptions{
				IsAirgap:                true,
				DisasterRecoveryEnabled: true,
				ServiceCIDR:             "10.96.0.0/12",
				AdminConsolePwd:         "password123",
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

				openEBS, ok := addons[0].(*openebs.OpenEBS)
				require.True(t, ok, "first addon should be OpenEBS")
				assert.Equal(t, "proxy.example.com", openEBS.ProxyRegistryDomain)

				eco, ok := addons[1].(*embeddedclusteroperator.EmbeddedClusterOperator)
				require.True(t, ok, "second addon should be EmbeddedClusterOperator")
				assert.True(t, eco.IsAirgap, "ECO should be in airgap mode")
				assert.Nil(t, eco.Proxy, "ECO should not have a proxy")
				assert.Empty(t, eco.ChartLocationOverride, "ECO should not have a chart location override")
				assert.Empty(t, eco.ChartVersionOverride, "ECO should not have a chart version override")
				assert.Empty(t, eco.ImageRepoOverride, "ECO should not have an image repo override")
				assert.Empty(t, eco.ImageTagOverride, "ECO should not have an image tag override")
				assert.Empty(t, eco.UtilsImageOverride, "ECO should not have a utils image override")
				assert.Equal(t, "proxy.example.com", eco.ProxyRegistryDomain)

				reg, ok := addons[2].(*registry.Registry)
				require.True(t, ok, "third addon should be Registry")
				assert.Equal(t, "10.96.0.0/12", reg.ServiceCIDR)
				assert.False(t, reg.IsHA, "Registry should not be in high availability mode")
				assert.Equal(t, "proxy.example.com", reg.ProxyRegistryDomain)

				vel, ok := addons[3].(*velero.Velero)
				require.True(t, ok, "fourth addon should be Velero")
				assert.Nil(t, vel.Proxy, "Velero should not have a proxy")
				assert.Equal(t, "proxy.example.com", vel.ProxyRegistryDomain)

				adminConsole, ok := addons[4].(*adminconsole.AdminConsole)
				require.True(t, ok, "fifth addon should be AdminConsole")
				assert.True(t, adminConsole.IsAirgap, "AdminConsole should be in airgap mode")
				assert.False(t, adminConsole.IsHA, "AdminConsole should not be in high availability mode")
				assert.Nil(t, adminConsole.Proxy, "AdminConsole should not have a proxy")
				assert.Equal(t, "10.96.0.0/12", adminConsole.ServiceCIDR)
				assert.Equal(t, "password123", adminConsole.Password)
				assert.Equal(t, "app.example.com", adminConsole.ReplicatedAppDomain)
				assert.Equal(t, "proxy.example.com", adminConsole.ProxyRegistryDomain)
				assert.Equal(t, "registry.example.com", adminConsole.ReplicatedRegistryDomain)
			},
		},
		{
			name: "with host CA bundle path",
			opts: InstallOptions{
				IsAirgap:                false,
				DisasterRecoveryEnabled: true, // Enable disaster recovery to also check Velero
				AdminConsolePwd:         "password123",
				HostCABundlePath:        "/etc/ssl/certs/ca-certificates.crt",
			},
			verify: func(t *testing.T, addons []types.AddOn) {
				// Find Velero and AdminConsole add-ons to verify HostCABundlePath
				var vel *velero.Velero
				var adminConsole *adminconsole.AdminConsole

				for _, addon := range addons {
					switch a := addon.(type) {
					case *velero.Velero:
						vel = a
					case *adminconsole.AdminConsole:
						adminConsole = a
					}
				}

				require.NotNil(t, vel, "Velero add-on should be present")
				require.NotNil(t, adminConsole, "AdminConsole add-on should be present")

				// Verify HostCABundlePath is properly passed
				assert.Equal(t, "/etc/ssl/certs/ca-certificates.crt", vel.HostCABundlePath,
					"Velero should have the correct HostCABundlePath")
				assert.Equal(t, "/etc/ssl/certs/ca-certificates.crt", adminConsole.HostCABundlePath,
					"AdminConsole should have the correct HostCABundlePath")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.before != nil {
				tt.before()
			}
			addons := getAddOnsForInstall(tt.opts)
			tt.verify(t, addons)
			if tt.after != nil {
				tt.after()
			}
		})
	}
}
