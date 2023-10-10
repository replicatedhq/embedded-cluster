package addons

import (
	"strings"

	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
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
