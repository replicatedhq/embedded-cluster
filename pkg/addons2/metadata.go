package addons2

import (
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/velero"
)

func Versions() map[string]string {
	addons := []types.AddOn{
		&openebs.OpenEBS{},
		&embeddedclusteroperator.EmbeddedClusterOperator{},
		&registry.Registry{},
		&seaweedfs.SeaweedFS{},
		&velero.Velero{},
		&adminconsole.AdminConsole{},
	}

	versions := map[string]string{}
	for _, addon := range addons {
		version := addon.Version()
		for k, v := range version {
			versions[k] = v
		}
	}

	return versions
}
