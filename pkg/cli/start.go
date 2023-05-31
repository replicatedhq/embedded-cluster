package cli //nolint:dupl

import (
	"github.com/k0sproject/k0s/cmd/start"
	"github.com/spf13/cobra"
)

// NewCmdStart returns a cobra command for starting the systemd service
func NewCmdStart(_ *CLI) *cobra.Command {
	return start.NewStartCmd()
}
