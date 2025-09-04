package helpers

import (
	"context"
	"io"
	"os/exec"
)

var h HelpersInterface

type Helpers struct{}

var _ HelpersInterface = (*Helpers)(nil)

func init() {
	Set(&Helpers{})
}

func Set(_h HelpersInterface) {
	h = _h
}

// HelpersInterface is an interface that wraps the RunCommand function.
type HelpersInterface interface {
	RunCommandWithOptions(opts RunCommandOptions, bin string, args ...string) error
	RunCommand(bin string, args ...string) (string, error)
	IsSystemdServiceActive(ctx context.Context, svcname string) (bool, error)
}

type RunCommandOptions struct {
	// Context is the context for the command.
	Context context.Context
	// Stdout is an additional io.Stdout to write the stdout of the command to.
	Stdout io.Writer
	// Stderr is an additional io.Stderr to write the stderr of the command to.
	Stderr io.Writer
	// Env is a map of additional environment variables to set for the command.
	Env map[string]string
	// Stdin is the standard input to be used when running the command.
	Stdin io.Reader
	// LogOnSuccess makes the command output to be logged even when it succeeds.
	LogOnSuccess bool
	// Cancel is a function to cancel the command.
	Cancel func(cmd *exec.Cmd)
}

// Convenience functions

func RunCommandWithOptions(opts RunCommandOptions, bin string, args ...string) error {
	return h.RunCommandWithOptions(opts, bin, args...)
}

func RunCommand(bin string, args ...string) (string, error) {
	return h.RunCommand(bin, args...)
}

func IsSystemdServiceActive(ctx context.Context, svcname string) (bool, error) {
	return h.IsSystemdServiceActive(ctx, svcname)
}
