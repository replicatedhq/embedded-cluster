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
		inSpec ecv1beta1.InstallationSpec
		before func()
		verify func(t *testing.T, addons []types.AddOn)
		after  func()
	}{
		{
			name: "online installation",
			inSpec: ecv1beta1.InstallationSpec{
				AirGap: false,
				LicenseInfo: &ecv1beta1.LicenseInfo{
					IsDisasterRecoverySupported: false,
				},
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
			name: "airgap installation",
			inSpec: ecv1beta1.InstallationSpec{
				AirGap: true,
				LicenseInfo: &ecv1beta1.LicenseInfo{
					IsDisasterRecoverySupported: false,
				},
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
			name: "online installation with disaster recovery enabled",
			inSpec: ecv1beta1.InstallationSpec{
				AirGap: false,
				LicenseInfo: &ecv1beta1.LicenseInfo{
					IsDisasterRecoverySupported: true,
				},
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
			name: "airgap installation with disaster recovery and proxy",
			inSpec: ecv1beta1.InstallationSpec{
				AirGap: true,
				LicenseInfo: &ecv1beta1.LicenseInfo{
					IsDisasterRecoverySupported: true,
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
			addons := getAddOnsForInstall(t.Logf, tt.inSpec)
			tt.verify(t, addons)
			if tt.after != nil {
				tt.after()
			}
		})
	}
}
