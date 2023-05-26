/*
Package cli provides the kurl command and its nested children.
*/
package cli

import (
	"os"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// CLI is the main CLI struct
type CLI struct {
	genericclioptions.IOStreams
	Args []string
}

// NewCLI creates a new CLI
func NewCLI() *CLI {
	return &CLI{
		Args: os.Args,
		IOStreams: genericclioptions.IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stderr,
		},
	}
}
