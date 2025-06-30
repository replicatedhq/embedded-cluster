package types

import (
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"k8s.io/client-go/rest"
)

// APIConfig holds the configuration for the API server
type APIConfig struct {
	Password      string
	TLSConfig     TLSConfig
	License       []byte
	AirgapBundle  string
	ConfigValues  string
	ReleaseData   *release.ReleaseData
	EndUserConfig *ecv1beta1.Config

	LinuxConfig
	KubernetesConfig
}

type LinuxConfig struct {
	RuntimeConfig             runtimeconfig.RuntimeConfig
	AllowIgnoreHostPreflights bool
}

type KubernetesConfig struct {
	RESTConfig   *rest.Config
	Installation kubernetesinstallation.Installation
}
