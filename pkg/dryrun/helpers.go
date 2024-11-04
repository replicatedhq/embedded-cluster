package dryrun

import (
	"bytes"
	"context"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
)

type Helpers struct{}

var _ helpers.HelpersInterface = (*Helpers)(nil)

func (h *Helpers) RunCommandWithOptions(opts helpers.RunCommandOptions, bin string, args ...string) error {
	RecordCommand(bin, args, opts.Env)
	return nil
}

func (h *Helpers) RunCommand(bin string, args ...string) (string, error) {
	stdout := bytes.NewBuffer(nil)
	if err := h.RunCommandWithOptions(helpers.RunCommandOptions{Stdout: stdout}, bin, args...); err != nil {
		return "", err
	}
	return stdout.String(), nil
}

func (h *Helpers) IsSystemdServiceActive(ctx context.Context, svcname string) (bool, error) {
	return false, nil
}
