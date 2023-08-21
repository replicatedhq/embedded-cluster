// Package prompt provides tooling around asking users for questions. This
// package chooses between "decorative" or "plain" prompts based on the
// environment variable HELMVM_PLAIN_PROMPTS. See 'decorative' and 'plain'
// packages for more information.
package prompts

import (
	"os"

	"github.com/replicatedhq/helmvm/pkg/prompts/decorative"
	"github.com/replicatedhq/helmvm/pkg/prompts/plain"
)

// Confirm asks a user for a "Yes" or "No" response. The default value
// is used if the user just presses enter without typing anything.
func Confirm(msg string, defvalue bool) bool {
	if os.Getenv("HELMVM_PLAIN_PROMPTS") == "true" {
		return plain.Confirm(msg, defvalue)
	}
	return decorative.Confirm(msg, defvalue)
}

// PressEnter asks the user to press enter to continue.
func PressEnter(msg string) {
	if os.Getenv("HELMVM_PLAIN_PROMPTS") == "true" {
		plain.PressEnter(msg)
		return
	}
	decorative.PressEnter(msg)
}

// Password asks the user for a password.
func Password(msg string) string {
	if os.Getenv("HELMVM_PLAIN_PROMPTS") == "true" {
		return plain.Password(msg)
	}
	return decorative.Password(msg)
}

// Select asks the user to select one of the provided options.
func Select(msg string, options []string, def string) string {
	if os.Getenv("HELMVM_PLAIN_PROMPTS") == "true" {
		return plain.Select(msg, options, def)
	}
	return decorative.Select(msg, options, def)
}

// Input asks the user for a string. If required is true then
// the string cannot be empty.
func Input(msg string, defvalue string, required bool) string {
	if os.Getenv("HELMVM_PLAIN_PROMPTS") == "true" {
		return plain.Input(msg, defvalue, required)
	}
	return decorative.Input(msg, defvalue, required)
}
