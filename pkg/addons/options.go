package addons

import (
	"os"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster-kinds/types"
)

// Option sets and option on an Applier reference.
type Option func(*Applier)

// WithoutPrompt disables the prompt before applying addons.
func WithoutPrompt() Option {
	return func(a *Applier) {
		a.prompt = false
	}
}

// Quiet disables logging for addons.
func Quiet() Option {
	return func(a *Applier) {
		a.verbose = false
	}
}

// WithConfig sets the helm config for the addons.
func WithConfig(config v1beta1.ClusterConfig) Option {
	return func(a *Applier) {
		a.config = config
	}
}

// OnlyDefaults causes addons to only return default values
func OnlyDefaults() Option {
	return func(a *Applier) {
		a.onlyDefaults = true
	}
}

// WithEndUserConfig sets the end user config passed in by the customer
// at install time. This configuration is similar to the one embedded
// in the cluster through a Kots Release.
func WithEndUserConfig(config *embeddedclusterv1beta1.Config) Option {
	return func(a *Applier) {
		a.endUserConfig = config
	}
}

// WithLicense sets the license for the application.
func WithLicense(licenseFile string) Option {
	return func(a *Applier) {
		a.licenseFile = licenseFile
	}
}

// WithAirgapBundle sets the airgap bundle for the application to be installed in airgap mode.
func WithAirgapBundle(airgapBundle string) Option {
	return func(a *Applier) {
		a.airgapBundle = airgapBundle
	}
}

// WithVersionMetadata sets the release version metadata to be used during addons installation.
func WithVersionMetadata(metadata *types.ReleaseMetadata) Option {
	return func(a *Applier) {
		a.releaseMetadata = metadata
	}
}

// WithProxyFromEnv sets the proxy environment variables to be used during addons installation.
func WithProxyFromEnv() Option {
	proxyEnv := map[string]string{
		"HTTP_PROXY":  os.Getenv("HTTP_PROXY"),
		"HTTPS_PROXY": os.Getenv("HTTPS_PROXY"),
		"NO_PROXY":    os.Getenv("NO_PROXY"),
	}

	return func(a *Applier) {
		a.proxyEnv = proxyEnv
	}
}
