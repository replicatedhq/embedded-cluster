package helm

import (
	"bytes"
	"context"
	"io"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
)

// BinaryExecutor is an interface for executing helm binary commands.
// This interface is mockable for testing purposes.
type BinaryExecutor interface {
	// ExecuteCommand runs a command and returns stdout, stderr, and error
	ExecuteCommand(ctx context.Context, env map[string]string, args ...string) (stdout string, stderr string, err error)
}

// binaryExecutor implements BinaryExecutor using helpers.RunCommandWithOptions
type binaryExecutor struct {
	bin   string // Path to the binary to execute
	logFn LogFn  // Optional logging function
}

// newBinaryExecutor creates a new binaryExecutor with the specified binary path
func newBinaryExecutor(bin string, logFn LogFn) BinaryExecutor {
	return &binaryExecutor{bin: bin, logFn: logFn}
}

// ExecuteCommand runs a command using helpers.RunCommandWithOptions and returns stdout, stderr, and error
func (c *binaryExecutor) ExecuteCommand(ctx context.Context, env map[string]string, args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	logWriter := &logWriter{logFn: c.logFn}

	err := helpers.RunCommandWithOptions(helpers.RunCommandOptions{
		Context: ctx,
		Stdout:  &stdout,
		Stderr:  io.MultiWriter(&stderr, logWriter), // Helm uses stderr for debug logging and progress
		Env:     env,
	}, c.bin, args...)

	return stdout.String(), stderr.String(), err
}

// logWriter wraps a logFn as an io.Writer
type logWriter struct {
	logFn LogFn
}

func (lw *logWriter) Write(p []byte) (n int, err error) {
	if lw.logFn != nil && len(p) > 0 {
		lw.logFn("%s", string(p))
	}
	return len(p), nil
}
