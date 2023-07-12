/*
Package cli provides the kurl command and its nested children.
*/
package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// CLI is the main CLI struct
type CLI struct {
	genericclioptions.IOStreams
	Name string
	Args []string
}

// NewCLI creates a new CLI
func NewCLI() *CLI {
	return &CLI{
		Name: executableName(os.Args),
		Args: os.Args,
		IOStreams: genericclioptions.IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stderr,
		},
	}
}

func (c *CLI) cmdReplaceK0s(cmd *cobra.Command) {
	replacer := strings.NewReplacer("k0s", c.Name)
	cmd.Use = replacer.Replace(cmd.Use)
	cmd.Short = replacer.Replace(cmd.Short)
	cmd.Long = replacer.Replace(cmd.Long)
	cmd.Example = replacer.Replace(cmd.Example)
}

func executableName(args []string) string {
	return filepath.Base(args[0])
}
