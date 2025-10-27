package helpers

import (
	"os"
	"strings"

	"github.com/spf13/afero"
)

var (
	_ HelpersInterface = &MockHelpers{}
)

// MockHelpers implements HelpersInterface for testing
type MockHelpers struct {
	Commands []string          // Track commands that were run
	Files    map[string][]byte // Track files that were written
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
	m.Files[path] = data
	return nil
}

func (m *MockHelpers) ReadFile(path string) ([]byte, error) {
	return m.Files[path], nil
}

func (m *MockHelpers) MoveFile(src, dst string) error {
	if _, ok := m.Files[src]; ok {
		m.Files[dst] = m.Files[src]
		delete(m.Files, src)
		return nil
	}
	return os.ErrNotExist
}

func (m *MockHelpers) Open(path string) (afero.File, error) {
	// TODO: fix this
	return nil, os.ErrNotExist
}

func (m *MockHelpers) OpenFile(path string, flag int, perm os.FileMode) (afero.File, error) {
	// TODO: fix this
	return nil, os.ErrNotExist
}

func (m *MockHelpers) Readdirnames(path string, n int) ([]string, error) {
	// TODO: implement this
	return nil, nil
}

func (m *MockHelpers) ReadDir(path string) ([]os.DirEntry, error) {
	// TODO: implement this
	return nil, nil
}

func (m *MockHelpers) Stat(path string) (os.FileInfo, error) {
	// TODO: implement this
	return nil, os.ErrNotExist
}

func (m *MockHelpers) Lstat(path string) (os.FileInfo, error) {
	// TODO: implement this
	return nil, os.ErrNotExist
}

func (m *MockHelpers) MkdirTemp(dir, pattern string) (string, error) {
	// TODO: implement this
	return "", nil
}

func (m *MockHelpers) CreateTemp(dir, pattern string) (afero.File, error) {
	// TODO: implement this
	return nil, nil
}

func (m *MockHelpers) RemoveAll(path string) error {
	// TODO: implement this
	return nil
}

func (m *MockHelpers) Remove(path string) error {
	// TODO: implement this
	return nil
}

func (m *MockHelpers) Chmod(path string, mode os.FileMode) error {
	// TODO: implement this
	return nil
}

func (m *MockHelpers) MkdirAll(path string, perm os.FileMode) error {
	// TODO: implement this
	return nil
}

func (m *MockHelpers) Rename(oldpath, newpath string) error {
	// TODO: implement this
	return nil
}
