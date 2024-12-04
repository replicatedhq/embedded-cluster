package types

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/pkg/addons2/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AddOn interface {
	Name() string
	Prepare() error
	Install(ctx context.Context, kcli client.Client, writer *spinner.MessageWriter) error
}

var _ AddOn = (*openebs.OpenEBS)(nil)
var _ AddOn = (*adminconsole.AdminConsole)(nil)
