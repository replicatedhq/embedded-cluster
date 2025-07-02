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
				ClusterID:               "123",
				IsAirgap:                false,
				DisasterRecoveryEnabled: false,
				AdminConsolePwd:         "password123",
			},
			verify: func(t *testing.T, addons []types.AddOn) {
				assert.Len(t, addons, 3)

				_, ok := addons[0].(*openebs.OpenEBS)
				require.True(t, ok, "first addon should be OpenEBS")

				eco, ok := addons[1].(*embeddedclusteroperator.EmbeddedClusterOperator)
				require.True(t, ok, "second addon should be EmbeddedClusterOperator")
				assert.False(t, eco.IsAirgap, "ECO should not be in airgap mode")
				assert.Nil(t, eco.Proxy, "ECO should not have a proxy")
				assert.Empty(t, eco.ChartLocationOverride, "ECO should not have a chart location override")
				assert.Empty(t, eco.ChartVersionOverride, "ECO should not have a chart version override")
				assert.Empty(t, eco.ImageRepoOverride, "ECO should not have an image repo override")
				assert.Empty(t, eco.ImageTagOverride, "ECO should not have an image tag override")
				assert.Empty(t, eco.UtilsImageOverride, "ECO should not have a utils image override")

				adminConsole, ok := addons[2].(*adminconsole.AdminConsole)
				require.True(t, ok, "third addon should be AdminConsole")
				assert.Equal(t, "123", adminConsole.ClusterID)
				assert.False(t, adminConsole.IsAirgap, "AdminConsole should not be in airgap mode")
				assert.False(t, adminConsole.IsHA, "AdminConsole should not be in high availability mode")
				assert.Nil(t, adminConsole.Proxy, "AdminConsole should not have a proxy")
				assert.Empty(t, adminConsole.ServiceCIDR, "AdminConsole should not have a service CIDR")
				assert.Equal(t, "password123", adminConsole.Password)
			},
		},
		{
			name: "airgap installation",
			opts: InstallOptions{
				ClusterID:               "123",
				IsAirgap:                true,
				DisasterRecoveryEnabled: false,
				AdminConsolePwd:         "password123",
				ServiceCIDR:             "10.96.0.0/12",
			},
			verify: func(t *testing.T, addons []types.AddOn) {
				assert.Len(t, addons, 4)

				_, ok := addons[0].(*openebs.OpenEBS)
				require.True(t, ok, "first addon should be OpenEBS")

				eco, ok := addons[1].(*embeddedclusteroperator.EmbeddedClusterOperator)
				require.True(t, ok, "second addon should be EmbeddedClusterOperator")
				assert.True(t, eco.IsAirgap, "ECO should be in airgap mode")
				assert.Nil(t, eco.Proxy, "ECO should not have a proxy")
				assert.Empty(t, eco.ChartLocationOverride, "ECO should not have a chart location override")
				assert.Empty(t, eco.ChartVersionOverride, "ECO should not have a chart version override")
				assert.Empty(t, eco.ImageRepoOverride, "ECO should not have an image repo override")
				assert.Empty(t, eco.ImageTagOverride, "ECO should not have an image tag override")
				assert.Empty(t, eco.UtilsImageOverride, "ECO should not have a utils image override")

				reg, ok := addons[2].(*registry.Registry)
				require.True(t, ok, "third addon should be Registry")
				assert.Equal(t, "10.96.0.0/12", reg.ServiceCIDR)

				adminConsole, ok := addons[3].(*adminconsole.AdminConsole)
				require.True(t, ok, "fourth addon should be AdminConsole")
				assert.Equal(t, "123", adminConsole.ClusterID)
				assert.True(t, adminConsole.IsAirgap, "AdminConsole should be in airgap mode")
				assert.False(t, adminConsole.IsHA, "AdminConsole should not be in high availability mode")
				assert.Nil(t, adminConsole.Proxy, "AdminConsole should not have a proxy")
				assert.Equal(t, "10.96.0.0/12", adminConsole.ServiceCIDR)
				assert.Equal(t, "password123", adminConsole.Password)
			},
		},
		{
			name: "disaster recovery enabled",
			opts: InstallOptions{
				ClusterID:               "123",
				IsAirgap:                false,
				DisasterRecoveryEnabled: true,
				AdminConsolePwd:         "password123",
				ServiceCIDR:             "10.96.0.0/12",
			},
			verify: func(t *testing.T, addons []types.AddOn) {
				assert.Len(t, addons, 4)

				_, ok := addons[0].(*openebs.OpenEBS)
				require.True(t, ok, "first addon should be OpenEBS")

				eco, ok := addons[1].(*embeddedclusteroperator.EmbeddedClusterOperator)
				require.True(t, ok, "second addon should be EmbeddedClusterOperator")
				assert.False(t, eco.IsAirgap, "ECO should not be in airgap mode")
				assert.Nil(t, eco.Proxy, "ECO should not have a proxy")
				assert.Empty(t, eco.ChartLocationOverride, "ECO should not have a chart location override")
				assert.Empty(t, eco.ChartVersionOverride, "ECO should not have a chart version override")
				assert.Empty(t, eco.ImageRepoOverride, "ECO should not have an image repo override")
				assert.Empty(t, eco.ImageTagOverride, "ECO should not have an image tag override")
				assert.Empty(t, eco.UtilsImageOverride, "ECO should not have a utils image override")

				vel, ok := addons[2].(*velero.Velero)
				require.True(t, ok, "third addon should be Velero")
				assert.Nil(t, vel.Proxy, "Velero should not have a proxy")

				adminConsole, ok := addons[3].(*adminconsole.AdminConsole)
				require.True(t, ok, "fourth addon should be AdminConsole")
				assert.Equal(t, "123", adminConsole.ClusterID)
				assert.False(t, eco.IsAirgap, "AdminConsole should not be in airgap mode")
				assert.False(t, adminConsole.IsHA, "AdminConsole should not be in high availability mode")
				assert.Nil(t, adminConsole.Proxy, "AdminConsole should not have a proxy")
				assert.Equal(t, "10.96.0.0/12", adminConsole.ServiceCIDR)
				assert.Equal(t, "password123", adminConsole.Password)
			},
		},
		{
			name: "airgap with disaster recovery and proxy",
			opts: InstallOptions{
				ClusterID:               "123",
				IsAirgap:                true,
				DisasterRecoveryEnabled: true,
				AdminConsolePwd:         "password123",
				ServiceCIDR:             "10.96.0.0/12",
				ProxySpec: &ecv1beta1.ProxySpec{
					HTTPProxy:  "http://proxy.example.com",
					HTTPSProxy: "https://proxy.example.com",
					NoProxy:    "localhost,127.0.0.1",
				},
			},
			verify: func(t *testing.T, addons []types.AddOn) {
				assert.Len(t, addons, 5)

				_, ok := addons[0].(*openebs.OpenEBS)
				require.True(t, ok, "first addon should be OpenEBS")

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

				reg, ok := addons[2].(*registry.Registry)
				require.True(t, ok, "third addon should be Registry")
				assert.Equal(t, "10.96.0.0/12", reg.ServiceCIDR)
				assert.False(t, reg.IsHA, "Registry should not be in high availability mode")

				vel, ok := addons[3].(*velero.Velero)
				require.True(t, ok, "fourth addon should be Velero")
				assert.Equal(t, "http://proxy.example.com", vel.Proxy.HTTPProxy)
				assert.Equal(t, "https://proxy.example.com", vel.Proxy.HTTPSProxy)
				assert.Equal(t, "localhost,127.0.0.1", vel.Proxy.NoProxy)

				adminConsole, ok := addons[4].(*adminconsole.AdminConsole)
				require.True(t, ok, "fifth addon should be AdminConsole")
				assert.Equal(t, "123", adminConsole.ClusterID)
				assert.True(t, adminConsole.IsAirgap, "AdminConsole should be in airgap mode")
				assert.False(t, adminConsole.IsHA, "AdminConsole should not be in high availability mode")
				assert.Equal(t, "http://proxy.example.com", adminConsole.Proxy.HTTPProxy)
				assert.Equal(t, "https://proxy.example.com", adminConsole.Proxy.HTTPSProxy)
				assert.Equal(t, "localhost,127.0.0.1", adminConsole.Proxy.NoProxy)
				assert.Equal(t, "10.96.0.0/12", adminConsole.ServiceCIDR)
				assert.Equal(t, "password123", adminConsole.Password)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.before != nil {
				tt.before()
			}
			tt.verify(t, GetAddOnsForInstall(tt.opts))
			if tt.after != nil {
				tt.after()
			}
		})
	}
}
