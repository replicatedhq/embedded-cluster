package cli

import (
	"bytes"
	"context"

	"github.com/spf13/cobra"
)

func testExecuteCommandC(ctx context.Context, root *cobra.Command, args ...string) (c *cobra.Command, output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	c, err = root.ExecuteContextC(ctx)
	return c, buf.String(), err
}
