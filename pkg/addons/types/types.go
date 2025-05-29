package types

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LogFunc func(format string, args ...interface{})

type AddOn interface {
	Name() string
	Version() string
	ReleaseName() string
	Namespace() string
	GenerateHelmValues(ctx context.Context, kcli client.Client, overrides []string) (map[string]interface{}, error)
	Install(ctx context.Context, logf LogFunc, kcli client.Client, hcli helm.Client, overrides []string, writer *spinner.MessageWriter) error
	Upgrade(ctx context.Context, logf LogFunc, kcli client.Client, hcli helm.Client, overrides []string) error
}
