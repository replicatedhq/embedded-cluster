package helm

import (
	"bytes"
	"context"
	"io"
	"maps"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
)

// BinaryExecutor is an interface for executing helm binary commands.
// This interface is mockable for testing purposes.
type BinaryExecutor interface {
	// ExecuteCommand runs a command and returns stdout, stderr, and error
	ExecuteCommand(ctx context.Context, env map[string]string, logFn LogFn, args ...string) (stdout string, stderr string, err error)
}

// binaryExecutor implements BinaryExecutor using helpers.RunCommandWithOptions
type binaryExecutor struct {
	bin        string            // Path to the binary to execute
	defaultEnv map[string]string // Default environment variables to set for all commands
}

// newBinaryExecutor creates a new binaryExecutor with the specified binary path and optional default environment
func newBinaryExecutor(bin string, homeDir string) BinaryExecutor {
	// Configure helm environment variables for tmpdir isolation
	helmEnv := map[string]string{
		"HELM_CACHE_HOME":  filepath.Join(homeDir, ".cache"),
		"HELM_CONFIG_HOME": filepath.Join(homeDir, ".config"),
		"HELM_DATA_HOME":   filepath.Join(homeDir, ".local"),
	}
	return &binaryExecutor{bin: bin, defaultEnv: helmEnv}
}

// ExecuteCommand runs a command using helpers.RunCommandWithOptions and returns stdout, stderr, and error
func (c *binaryExecutor) ExecuteCommand(ctx context.Context, env map[string]string, logFn LogFn, args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	logWriter := &logWriter{logFn: logFn}

	// Merge default environment with provided environment (provided env takes precedence)
	mergedEnv := make(map[string]string)
	maps.Copy(mergedEnv, c.defaultEnv)
	maps.Copy(mergedEnv, env)

	err := helpers.RunCommandWithOptions(helpers.RunCommandOptions{
		Context: ctx,
		Stdout:  &stdout,
		Stderr:  io.MultiWriter(&stderr, logWriter), // Helm uses stderr for debug logging and progress
		Env:     mergedEnv,
	}, c.bin, args...)

	return stdout.String(), stderr.String(), err
}

// logWriter wraps a logFn as an io.Writer
type logWriter struct {
	logFn LogFn
}

// match log lines that come from go files to reduce noise and keep the logs relevant and readable to the user
var goFilePattern = regexp.MustCompile(`^\w+\.go:\d+:`)

func (lw *logWriter) Write(p []byte) (n int, err error) {
	if lw.logFn != nil && len(p) > 0 {
		line := strings.TrimSpace(string(p))
		if line != "" && goFilePattern.MatchString(line) {
			lw.logFn("helm: %s", line)
		}
	}
	return len(p), nil
}
