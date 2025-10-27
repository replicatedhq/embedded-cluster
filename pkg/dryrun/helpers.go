package dryrun

import (
	"bytes"
	"os"

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

func (h *Helpers) WriteFile(path string, data []byte, perm os.FileMode) error {
	RecordFileWrite(path, data, perm)
	return nil
}
