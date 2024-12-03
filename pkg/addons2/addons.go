package addons2

import (
	"context"

	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/types"
)

// this is a temp function that's much more specific than we actually need it to be
// this is going to get us to working installs, and we refactor.
// this is not configurable at all, it's not the way it needs to be in the product
func InstallDefaultAddons(ctx context.Context, clusterConfig *k0sconfig.ClusterConfig) error {
	addsOns := []types.AddOn{
		&openebs.OpenEBS{},
	}

	for _, addon := range addsOns {
		if err := addon.Install(ctx, clusterConfig); err != nil {
			return err
		}
	}

	return nil
}
