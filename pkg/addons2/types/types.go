package types

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/pkg/addons2/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/velero"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AddOn interface {
	Name() string
	Install(ctx context.Context, kcli client.Client, writer *spinner.MessageWriter) error
}

var _ AddOn = (*adminconsole.AdminConsole)(nil)
var _ AddOn = (*openebs.OpenEBS)(nil)
var _ AddOn = (*registry.Registry)(nil)
var _ AddOn = (*seaweedfs.SeaweedFS)(nil)
var _ AddOn = (*velero.Velero)(nil)
