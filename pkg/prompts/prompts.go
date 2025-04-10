// Package prompts provides tooling around asking users for questions. This
// package chooses between "decorative" or "plain" prompts based on the
// environment variable EMBEDDED_CLUSTER_PLAIN_PROMPTS. See 'decorative' and 'plain'
// packages for more information.
package prompts

import (
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/prompts/decorative"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts/plain"
)

var (
	_isTerminal bool = false
)

func SetTerminal(isTerminal bool) {
	_isTerminal = isTerminal
}

// Prompt is the interface implemented by 'decorative' and 'plain' prompts.
type Prompt interface {
	// Confirm asks for user for a "Yes" or "No" response. The default value is used if the user
	// presses enter without typing anything.
	Confirm(msg string, defvalue bool) (bool, error)
	// PressEnter asks the user to press enter to continue.
	PressEnter(msg string) error
	// Password asks the user for a password. Password can't be empty.
	Password(msg string) (string, error)
	// Select asks the user to select one of the provided options.
	Select(msg string, options []string, defvalue string) (string, error)
	// Input asks the user for a string. If required is true then the string cannot be empty.
	Input(msg string, defvalue string, required bool) (string, error)
}

// New returns a new Prompt.
func New() Prompt {
	if os.Getenv("EMBEDDED_CLUSTER_PLAIN_PROMPTS") == "true" {
		return plain.New()
	}
	return decorative.New()
}

func IsTerminal() bool {
	return _isTerminal
}
