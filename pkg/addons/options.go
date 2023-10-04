package addons

import "strings"

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

// IsUpgrade sets the applier to upgrade addons.
func IsUpgrade() Option {
	return func(a *Applier) {
		a.isUpgrade = true
	}
}

// Quiet disables logging for addons.
func Quiet() Option {
	return func(a *Applier) {
		a.verbose = false
	}
}
