package types

import (
	"context"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/velero"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AddOn interface {
	Name() string
	Version() map[string]string
	ReleaseName() string
	Namespace() string
	GetImages() []string
	GetAdditionalImages() []string
	GenerateChartConfig() ([]ecv1beta1.Chart, []k0sv1beta1.Repository, error)
	Install(ctx context.Context, kcli client.Client, hcli *helm.Helm, overrides []string, writer *spinner.MessageWriter) error
	Upgrade(ctx context.Context, kcli client.Client, hcli *helm.Helm, overrides []string) error
}

var _ AddOn = (*adminconsole.AdminConsole)(nil)
var _ AddOn = (*openebs.OpenEBS)(nil)
var _ AddOn = (*registry.Registry)(nil)
var _ AddOn = (*seaweedfs.SeaweedFS)(nil)
var _ AddOn = (*velero.Velero)(nil)
var _ AddOn = (*embeddedclusteroperator.EmbeddedClusterOperator)(nil)
