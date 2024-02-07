package addons

import (
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster-operator/api/v1beta1"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
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
func WithLicense(license *kotsv1beta1.License) Option {
	return func(a *Applier) {
		a.license = license
	}
}
