package types

import "github.com/replicatedhq/embedded-cluster/pkg/addons2/openebs"

type AddOn interface {
	Install() error
	Upgrade() error
}

var _ AddOn = (*openebs.OpenEBS)(nil)
