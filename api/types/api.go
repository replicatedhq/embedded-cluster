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
	Password           string // Used by AdminConsole addon to create kotsadm-password secret and infrastructure operations in V2
	PasswordHash       []byte // Used by auth controller for API authentication in V3
	TLSConfig          TLSConfig
	License            []byte
	AirgapBundle       string
	AirgapMetadata     *airgap.AirgapMetadata
	EmbeddedAssetsSize int64
	ConfigValues       AppConfigValues
	ReleaseData        *release.ReleaseData
	EndUserConfig      *ecv1beta1.Config
	ClusterID          string
	Mode               Mode

	LinuxConfig
	KubernetesConfig
}

type Mode string

const (
	ModeUpgrade Mode = "upgrade"
	ModeInstall Mode = "install"
)

type LinuxConfig struct {
	RuntimeConfig             runtimeconfig.RuntimeConfig
	AllowIgnoreHostPreflights bool
}

type KubernetesConfig struct {
	RESTClientGetter genericclioptions.RESTClientGetter
	Installation     kubernetesinstallation.Installation
}
