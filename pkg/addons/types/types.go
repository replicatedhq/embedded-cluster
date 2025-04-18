package types

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/velero"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AddOn interface {
	Name() string
	Version() string
	ReleaseName() string
	Namespace() string
	GenerateHelmValues(ctx context.Context, kcli client.Client, overrides []string) (map[string]interface{}, error)
	Install(ctx context.Context, kcli client.Client, hcli helm.Client, overrides []string, writer *spinner.MessageWriter) error
	Upgrade(ctx context.Context, kcli client.Client, hcli helm.Client, overrides []string) error
}

var _ AddOn = (*adminconsole.AdminConsole)(nil)
var _ AddOn = (*openebs.OpenEBS)(nil)
var _ AddOn = (*registry.Registry)(nil)
var _ AddOn = (*seaweedfs.SeaweedFS)(nil)
var _ AddOn = (*velero.Velero)(nil)
var _ AddOn = (*embeddedclusteroperator.EmbeddedClusterOperator)(nil)
