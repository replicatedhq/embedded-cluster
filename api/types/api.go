package types

import (
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// APIConfig holds the configuration for the API server
type APIConfig struct {
	InstallTarget        InstallTarget
	Password             string // Used by AdminConsole addon to create kotsadm-password secret and infrastructure operations in V2
	PasswordHash         []byte // Used by auth controller for API authentication in V3
	TLSConfig            TLSConfig
	License              []byte
	AirgapBundle         string
	AirgapMetadata       *airgap.AirgapMetadata
	EmbeddedAssetsSize   int64
	ConfigValues         AppConfigValues
	ReleaseData          *release.ReleaseData
	EndUserConfig        *ecv1beta1.Config
	ClusterID            string
	Mode                 Mode
	TargetVersion        string // Used for upgrade metrics reporting
	InitialVersion       string // Used for upgrade metrics reporting
	RequiresInfraUpgrade bool   // Whether an infrastructure upgrade is required (only relevant for upgrades)

	LinuxConfig
	KubernetesConfig
	MigrationConfig
}

type InstallTarget string

const (
	InstallTargetLinux      InstallTarget = "linux"
	InstallTargetKubernetes InstallTarget = "kubernetes"
)

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
	Installation kubernetesinstallation.Installation
}

type MigrationConfig struct {
	KubeClient       client.Client // For interacting with kURL cluster
	KurlPasswordHash string        // Bcrypt hash exported from kURL
}
