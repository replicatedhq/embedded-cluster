package helpers

import (
	"os"
	"strings"
)

var (
	_ HelpersInterface = &MockHelpers{}
)

// MockHelpers implements HelpersInterface for testing
type MockHelpers struct {
	Commands []string // Track commands that were run
	Files    []string // Track files that were written
}

func (m *MockHelpers) RunCommandWithOptions(opts RunCommandOptions, bin string, args ...string) error {
	m.Commands = append(m.Commands, bin+" "+strings.Join(args, " "))
	return nil
}

func (m *MockHelpers) RunCommand(bin string, args ...string) (string, error) {
	m.Commands = append(m.Commands, bin+" "+strings.Join(args, " "))
	return "", nil
}

func (m *MockHelpers) WriteFile(path string, data []byte, perm os.FileMode) error {
	m.Files = append(m.Files, path)
	return nil
}
