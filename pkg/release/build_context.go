package release

import (
	"context"
	"debug/elf"
	"fmt"
	"io"
	"os"
	"path"
)

// buildContext holds the context for a release build.
type buildContext struct {
	context.Context
	baseDir string
	request *BuildRequest
	result  *os.File
}

// Close ends the build context. Removes anything created during the build.
func (b *buildContext) Close() error {
	if b.baseDir == "" {
		return nil
	}
	if b.result != nil {
		b.result.Close()
	}
	return os.RemoveAll(b.baseDir)
}

// Size returns the built release size.
func (b *buildContext) Size() int64 {
	finfo, err := os.Stat(b.tgzPath())
	if err != nil {
		return 0
	}
	return finfo.Size()
}

// Read reads the release file from the build context.
func (b *buildContext) Read(to []byte) (n int, err error) {
	if b.result == nil {
		var err error
		if b.result, err = os.Open(b.tgzPath()); err != nil {
			return 0, fmt.Errorf("unable to open binary: %v", err)
		}
	}
	return b.result.Read(to)
}

// tgzPath returns the path to the tgz file where the release is stored.
func (b *buildContext) tgzPath() string {
	fname := fmt.Sprintf("%s.tgz", b.request.uniqName())
	return path.Join(b.baseDir, fname)
}

// binaryPath returns the path to the binary.
func (b *buildContext) binaryPath() string {
	return path.Join(b.baseDir, b.request.BinaryName)
}

// kotsReleasePath returns the path to the KOTS release.
func (b *buildContext) kotsReleasePath() string {
	return path.Join(b.baseDir, "release.tar.gz")
}

// kotsReleaseObjectPath returns the path to the KOTS release.
func (b *buildContext) kotsReleaseObjectPath() string {
	return fmt.Sprintf("%s.o", b.kotsReleasePath())
}

// withSourceBinary writes the source binary to the build context.
func (b *buildContext) withSourceBinary(src io.Reader) error {
	dst, err := os.Create(b.binaryPath())
	if err != nil {
		return fmt.Errorf("unable to create binary: %v", err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("unable to write binary: %v", err)
	}
	if err := os.Chmod(b.binaryPath(), 0755); err != nil {
		return fmt.Errorf("unable to chmod binary: %v", err)
	}
	return nil
}

// hasEmbeddedRelease returns if the binary already has a release embedded.
// Here we look for the sec_bundle section.
func (b *buildContext) hasEmbeddedRelease() (bool, error) {
	bin, err := elf.Open(b.binaryPath())
	if err != nil {
		return false, fmt.Errorf("unable to open binary: %v", err)
	}
	section := bin.Section("sec_bundle")
	return section != nil, nil
}

// newBuildContext returns a new build context.
func newBuildContext(ctx context.Context, req *BuildRequest) (*buildContext, error) {
	tmpdir, err := os.MkdirTemp("", fmt.Sprintf("%s-*", req.uniqName()))
	if err != nil {
		return nil, fmt.Errorf("unable to create temp dir: %v", err)
	}
	fp, err := os.Create(path.Join(tmpdir, "release.tar.gz"))
	if err != nil {
		_ = os.RemoveAll(tmpdir)
		return nil, fmt.Errorf("unable to create release: %v", err)
	}
	defer fp.Close()
	if _, err := io.Copy(fp, req.kotsReleaseReader()); err != nil {
		_ = os.RemoveAll(tmpdir)
		return nil, fmt.Errorf("unable to write release: %v", err)
	}
	return &buildContext{
		Context: ctx,
		request: req,
		baseDir: tmpdir,
	}, nil
}
