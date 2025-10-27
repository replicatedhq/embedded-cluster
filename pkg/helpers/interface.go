package helpers

import (
	"context"
	"io"
	"os"

	"github.com/spf13/afero"
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
	WriteFile(path string, data []byte, perm os.FileMode) error
	ReadFile(path string) ([]byte, error)
	MoveFile(src, dst string) error
	Open(path string) (afero.File, error)
	OpenFile(path string, flag int, perm os.FileMode) (afero.File, error)
	ReadDir(path string) ([]os.DirEntry, error)
	Stat(path string) (os.FileInfo, error)
	Lstat(path string) (os.FileInfo, error)
	MkdirTemp(dir, pattern string) (string, error)
	CreateTemp(dir, pattern string) (afero.File, error)
	RemoveAll(path string) error
	Remove(path string) error
	Chmod(path string, mode os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Rename(oldpath, newpath string) error
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
}

// Convenience functions

func RunCommandWithOptions(opts RunCommandOptions, bin string, args ...string) error {
	return h.RunCommandWithOptions(opts, bin, args...)
}

func RunCommand(bin string, args ...string) (string, error) {
	return h.RunCommand(bin, args...)
}

func WriteFile(path string, data []byte, perm os.FileMode) error {
	return h.WriteFile(path, data, perm)
}

func ReadFile(path string) ([]byte, error) {
	return h.ReadFile(path)
}

func MoveFile(src, dst string) error {
	return h.MoveFile(src, dst)
}

func Open(path string) (afero.File, error) {
	return h.Open(path)
}

func OpenFile(path string, flag int, perm os.FileMode) (afero.File, error) {
	return h.OpenFile(path, flag, perm)
}

func ReadDir(path string) ([]os.DirEntry, error) {
	return h.ReadDir(path)
}

func Stat(path string) (os.FileInfo, error) {
	return h.Stat(path)
}

func Lstat(path string) (os.FileInfo, error) {
	return h.Lstat(path)
}

func MkdirTemp(dir, pattern string) (string, error) {
	return h.MkdirTemp(dir, pattern)
}

func CreateTemp(dir, pattern string) (afero.File, error) {
	return h.CreateTemp(dir, pattern)
}

func Remove(path string) error {
	return h.Remove(path)
}

func Chmod(path string, mode os.FileMode) error {
	return h.Chmod(path, mode)
}

func MkdirAll(path string, perm os.FileMode) error {
	return h.MkdirAll(path, perm)
}

func Rename(oldpath, newpath string) error {
	return h.Rename(oldpath, newpath)
}
