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
	_isTerminal bool   = false
	_testPrompt Prompt = nil
	_isHeadless bool   = false
)

func SetTerminal(isTerminal bool) {
	_isTerminal = isTerminal
}

func SetHeadless(isHeadless bool) {
	_isHeadless = isHeadless
}

// Prompt is the interface implemented by 'decorative' and 'plain' prompts.
type Prompt interface {
	// Confirm asks for user for a "Yes" or "No" response. The default value is used if the user
	// presses enter without typing anything.
	Confirm(msg string, defvalue bool) (bool, error)
	// Password asks the user for a password. Password can't be empty.
	Password(msg string) (string, error)
	// Input asks the user for a string. If required is true then the string cannot be empty.
	Input(msg string, defvalue string, required bool) (string, error)
}

// New returns a new Prompt.
func New() Prompt {
	// If a test prompt is set, use it instead
	if _testPrompt != nil {
		return _testPrompt
	}
	// If in headless mode, return a prompt with no stdin
	if _isHeadless {
		return plain.New(plain.WithIn(nil))
	}
	if os.Getenv("EMBEDDED_CLUSTER_PLAIN_PROMPTS") == "true" {
		return plain.New()
	}
	return decorative.New()
}

// SetTestPrompt sets a test prompt to be used instead of the normal prompt.
// This is intended for testing purposes only.
func SetTestPrompt(prompt Prompt) {
	_testPrompt = prompt
}

// ClearTestPrompt clears the test prompt and restores normal behavior.
func ClearTestPrompt() {
	_testPrompt = nil
}

func IsTerminal() bool {
	return _isTerminal
}
