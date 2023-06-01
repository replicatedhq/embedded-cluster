package cli //nolint:dupl

import (
	"github.com/k0sproject/k0s/cmd/stop"
	"github.com/spf13/cobra"
)

// NewCmdStop returns a cobra command for stopping the systemd service
func NewCmdStop(_ *CLI) *cobra.Command {
	return stop.NewStopCmd()
}
