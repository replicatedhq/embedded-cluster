package types

import (
	"context"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LogFunc func(format string, args ...interface{})

type AddOn interface {
	Name() string
	Version() string
	ReleaseName() string
	Namespace() string
	GenerateHelmValues(ctx context.Context, inSpec ecv1beta1.InstallationSpec, overrides []string) (map[string]interface{}, error)
	Install(ctx context.Context, clients Clients, writer *spinner.MessageWriter, inSpec ecv1beta1.InstallationSpec, overrides []string, installOpts InstallOptions) error
	// TODO: message writer for enable HA
	Upgrade(ctx context.Context, clients Clients, inSpec ecv1beta1.InstallationSpec, overrides []string) error
}

type Clients struct {
	K8sClient      client.Client
	MetadataClient metadata.Interface
	HelmClient     helm.Client
	IsDryRun       bool
}

type InstallOptions struct {
	AdminConsolePassword string
	TLSCertBytes         []byte
	TLSKeyBytes          []byte
	Hostname             string
	EndUserConfigSpec    *ecv1beta1.ConfigSpec
	KotsInstaller        KotsInstaller
	IsRestore            bool
}

type KotsInstaller func(msg *spinner.MessageWriter) error
