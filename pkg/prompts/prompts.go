// Package prompts provides tooling around asking users for questions. This
// package chooses between "decorative" or "plain" prompts based on the
// environment variable HELMVM_PLAIN_PROMPTS. See 'decorative' and 'plain'
// packages for more information.
package prompts

import (
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/prompts/decorative"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts/plain"
)

// Prompt is the interface implemented by 'decorative' and 'plain' prompts.
type Prompt interface {
	Confirm(string, bool) bool
	PressEnter(string)
	Password(string) string
	Select(string, []string, string) string
	Input(string, string, bool) string
}

// New returns a new Prompt.
func New() Prompt {
	if os.Getenv("HELMVM_PLAIN_PROMPTS") == "true" {
		return plain.Plain{}
	}
	return decorative.Decorative{}
}
