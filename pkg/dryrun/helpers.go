package dryrun

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
)

var _ helpers.HelpersInterface = (*Helpers)(nil)

type Helpers struct {
	stubs []CommandStub
}

// CommandStub allows tests to inject command behavior at runtime.
// If Match returns true for a given binary and args, Respond is invoked
// and its returned string is written to Stdout (if provided).
type CommandStub struct {
	// Pattern matches against the entire command string:
	// "<bin> <arg1> <arg2> ..."
	Pattern *regexp.Regexp
	Respond func(args []string) (string, error)
}

// SetCommandStubs replaces the command stubs used by this Helpers instance.
func (h *Helpers) SetCommandStubs(stubs []CommandStub) {
	h.stubs = append([]CommandStub(nil), stubs...)
}

func (h *Helpers) RunCommandWithOptions(opts helpers.RunCommandOptions, bin string, args ...string) error {
	RecordCommand(bin, args, opts.Env)

	// Allow test-injected command stubs to handle this invocation.
	if h != nil {
		full := strings.TrimSpace(filepath.Base(bin) + " " + strings.Join(args, " "))
		for _, stub := range h.stubs {
			if stub.Pattern == nil || !stub.Pattern.MatchString(full) {
				continue
			}
			if stub.Respond == nil {
				return nil
			}
			out, err := stub.Respond(args)
			if opts.Stdout != nil && out != "" {
				_, _ = io.WriteString(opts.Stdout, out)
			}
			return err
		}
	}

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
