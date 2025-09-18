package types

import (
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// APIConfig holds the configuration for the API server
type APIConfig struct {
	Password           string
	TLSConfig          TLSConfig
	License            []byte
	AirgapBundle       string
	AirgapMetadata     *airgap.AirgapMetadata
	EmbeddedAssetsSize int64
	ConfigValues       AppConfigValues
	ReleaseData        *release.ReleaseData
	EndUserConfig      *ecv1beta1.Config
	ClusterID          string

	LinuxConfig
	KubernetesConfig
}

type LinuxConfig struct {
	RuntimeConfig             runtimeconfig.RuntimeConfig
	AllowIgnoreHostPreflights bool
}

type KubernetesConfig struct {
	RESTClientGetter genericclioptions.RESTClientGetter
	Installation     kubernetesinstallation.Installation
}
