package addons

import (
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
)

// Option sets and option on an Applier reference.
type Option func(*Applier)

// WithoutPrompt disables the prompt before applying addons.
func WithoutPrompt() Option {
	return func(a *Applier) {
		a.prompt = false
	}
}

// WithPrivateCAs sets the private CAs to be used during addons installation.
func WithPrivateCAs(privateCAs map[string]string) Option {
	return func(a *Applier) {
		a.privateCAs = privateCAs
	}
}

// Quiet disables logging for addons.
func Quiet() Option {
	return func(a *Applier) {
		a.verbose = false
	}
}

// OnlyDefaults causes addons to only return default values
func OnlyDefaults() Option {
	return func(a *Applier) {
		a.onlyDefaults = true
		a.runtimeConfig = ecv1beta1.GetDefaultRuntimeConfig()
		a.provider = defaults.NewProviderFromRuntimeConfig(a.runtimeConfig)
	}
}

// WithEndUserConfig sets the end user config passed in by the customer
// at install time. This configuration is similar to the one embedded
// in the cluster through a Kots Release.
func WithEndUserConfig(config *ecv1beta1.Config) Option {
	return func(a *Applier) {
		a.endUserConfig = config
	}
}

// WithRuntimeConfig sets the runtime config passed in by the customer
// at install time.
func WithRuntimeConfig(runtimeConfig *ecv1beta1.RuntimeConfigSpec) Option {
	return func(a *Applier) {
		a.runtimeConfig = runtimeConfig
		a.provider = defaults.NewProviderFromRuntimeConfig(runtimeConfig)
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

// WithProxy sets the proxy environment variables to be used during addons installation.
func WithProxy(httpProxy string, httpsProxy string, noProxy string) Option {
	proxyEnv := map[string]string{
		"HTTP_PROXY":  httpProxy,
		"HTTPS_PROXY": httpsProxy,
		"NO_PROXY":    noProxy,
	}

	return func(a *Applier) {
		a.proxyEnv = proxyEnv
	}
}

// WithAdminConsolePassword sets the password for the admin console
func WithAdminConsolePassword(password string) Option {
	return func(a *Applier) {
		a.adminConsolePwd = password
	}
}
