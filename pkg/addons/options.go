package addons

import (
	"strings"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster-operator/api/v1beta1"
)

// Option sets and option on an Applier reference.
type Option func(*Applier)

// WithoutAddon disables an addon from being applied.
func WithoutAddon(addon string) Option {
	addon = strings.ToLower(addon)
	return func(a *Applier) {
		a.disabledAddons[addon] = true
	}
}

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
