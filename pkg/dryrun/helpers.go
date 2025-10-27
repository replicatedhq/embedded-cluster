package dryrun

import (
	"bytes"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/spf13/afero"
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
	return WriteFile(path, data, perm)
}

func (h *Helpers) ReadFile(path string) ([]byte, error) {
	return ReadFile(path)
}

func (h *Helpers) MoveFile(src, dst string) error {
	return MoveFile(src, dst)
}

func (h *Helpers) Open(path string) (afero.File, error) {
	return Open(path)
}

func (h *Helpers) OpenFile(path string, flag int, perm os.FileMode) (afero.File, error) {
	return OpenFile(path, flag, perm)
}

func (h *Helpers) ReadDir(path string) ([]os.DirEntry, error) {
	return ReadDir(path)
}

func (h *Helpers) Stat(path string) (os.FileInfo, error) {
	return Stat(path)
}

func (h *Helpers) Lstat(path string) (os.FileInfo, error) {
	return Lstat(path)
}

func (h *Helpers) MkdirTemp(dir, pattern string) (string, error) {
	return MkdirTemp(dir, pattern)
}

func (h *Helpers) CreateTemp(dir, pattern string) (afero.File, error) {
	return CreateTemp(dir, pattern)
}

func (h *Helpers) RemoveAll(path string) error {
	return RemoveAll(path)
}

func (h *Helpers) Remove(path string) error {
	return Remove(path)
}

func (h *Helpers) Chmod(path string, mode os.FileMode) error {
	return Chmod(path, mode)
}

func (h *Helpers) MkdirAll(path string, perm os.FileMode) error {
	return MkdirAll(path, perm)
}

func (h *Helpers) Rename(oldpath, newpath string) error {
	return Rename(oldpath, newpath)
}
