package types

import (
	"context"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LogFunc func(format string, args ...interface{})

type AddOn interface {
	Name() string
	Version() string
	ReleaseName() string
	Namespace() string
	GenerateHelmValues(ctx context.Context, kcli client.Client, domains ecv1beta1.Domains, overrides []string) (map[string]interface{}, error)
	Install(ctx context.Context, logf LogFunc, kcli client.Client, mcli metadata.Interface, hcli helm.Client, domains ecv1beta1.Domains, overrides []string) error
	Upgrade(ctx context.Context, logf LogFunc, kcli client.Client, mcli metadata.Interface, hcli helm.Client, domains ecv1beta1.Domains, overrides []string) error
}

type AddOnProgress struct {
	Name   string
	Status apitypes.Status
}
