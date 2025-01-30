package addons2

import (
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/velero"
)

func Versions() map[string]string {
	versions := map[string]string{}
	for _, addon := range getAddOnsForMetadata() {
		version := addon.Version()
		for k, v := range version {
			versions[k] = v
		}
	}

	return versions
}

func GenerateChartConfigs() ([]ecv1beta1.Chart, []k0sv1beta1.Repository, error) {
	charts := []ecv1beta1.Chart{}
	repositories := []k0sv1beta1.Repository{}

	for _, addon := range getAddOnsForMetadata() {
		chart, repos, err := addon.GenerateChartConfig()
		if err != nil {
			return nil, nil, err
		}
		charts = append(charts, chart...)
		repositories = append(repositories, repos...)
	}

	return charts, repositories, nil
}

func GetImages() []string {
	var images []string
	for _, addon := range getAddOnsForMetadata() {
		images = append(images, addon.GetImages()...)
	}
	return images
}

func GetAdditionalImages() []string {
	var images []string
	for _, addon := range getAddOnsForMetadata() {
		images = append(images, addon.GetAdditionalImages()...)
	}
	return images
}

func getAddOnsForMetadata() []types.AddOn {
	return []types.AddOn{
		&openebs.OpenEBS{},
		&embeddedclusteroperator.EmbeddedClusterOperator{},
		&registry.Registry{},
		&seaweedfs.SeaweedFS{},
		&velero.Velero{},
		&adminconsole.AdminConsole{},
	}
}
