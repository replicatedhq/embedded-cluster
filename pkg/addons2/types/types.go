package types

import (
	"context"

	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/openebs"
)

type AddOn interface {
	Install(ctx context.Context, clusterConfig *k0sconfig.ClusterConfig) error
}

var _ AddOn = (*openebs.OpenEBS)(nil)
