package embed

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
)

// ElfBuilder is an implementation of the Embedded Cluster Builder using elf sections. This
// is deprecated and exists for backward compatibility. Used in old Embedded Cluster versions.
type ElfBuilder struct{ baseDir string }

// NewElfBuilder returns a new instance of the ElfBuilder. Callers need to Close the builder
// when done with it, this ensure temporary files are removed.
func NewElfBuilder(tmpdir string) (*ElfBuilder, error) {
	if _, err := exec.LookPath("objcopy"); err != nil {
		return nil, fmt.Errorf("unable to find objcopy binary in path: %w", err)
	}
	tmp, err := os.MkdirTemp(tmpdir, "embedded-cluster-builder-*")
	if err != nil {
		return nil, fmt.Errorf("unable to create temp dir: %w", err)
	}
	return &ElfBuilder{tmp}, nil
}

// Build builds a new version of the Embedded Cluster binary pointed by bin with the release
// pointed by release embedded into it. Returns a ReadCloser from where the new binary can be
// read from. Callers need to Close the returned value when done with it.
func (e *ElfBuilder) Build(bin, release string) (io.ReadCloser, error) {
	if err := e.withBinary(bin); err != nil {
		return nil, fmt.Errorf("unable to add binary to builder: %w", err)
	}
	if err := e.withRelease(release); err != nil {
		return nil, fmt.Errorf("unable to add release to builder: %w", err)
	}
	if err := e.embed(); err != nil {
		return nil, fmt.Errorf("unable to build binary: %w", err)
	}
	binpath := path.Join(e.baseDir, "embedded-cluster")
	return os.Open(binpath)
}

// Size returns the size of the embedded cluster binary once it is built (includes the kots
// release data).
func (e *ElfBuilder) Size() (int64, error) {
	binpath := path.Join(e.baseDir, "embedded-cluster")
	fi, err := os.Stat(binpath)
	if err != nil {
		return 0, fmt.Errorf("unable to stat binary: %w", err)
	}
	return fi.Size(), nil
}

// Close closes the builder. Removes the temporary directory.
func (e *ElfBuilder) Close() error {
	return os.RemoveAll(e.baseDir)
}

// embed embeds the release object file into the embedded cluster binary in a section called
// sec_bundle.
func (e *ElfBuilder) embed() error {
	binary := path.Join(e.baseDir, "embedded-cluster")
	release := path.Join(e.baseDir, "release.o")
	bundleArg := fmt.Sprintf("sec_bundle=%s", release)
	command := exec.Command("objcopy", "--add-section", bundleArg, binary)
	if out, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("unable to embed release: %v: %s", err, string(out))
	}
	return nil
}

// withBinary adds the given binary to the builder directory. This copies the binary to the
// temporary directory as the elf builder needs to modify it.
func (e *ElfBuilder) withBinary(bin string) error {
	src, err := os.Open(bin)
	if err != nil {
		return fmt.Errorf("unable to open binary file: %w", err)
	}
	defer src.Close()
	dstpath := path.Join(e.baseDir, "embedded-cluster")
	dst, err := os.Create(dstpath)
	if err != nil {
		return fmt.Errorf("unable to create destination file: %w", err)
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

// WithRelease adds the given release to the builder. Turns the release into an object file.
func (c *ElfBuilder) withRelease(release string) error {
	dstpath := path.Join(c.baseDir, "release.o")
	command := exec.Command(
		"objcopy",
		"--input-target", "binary",
		"--output-target", "binary",
		"--rename-section", ".data=sec_bundle",
		release, dstpath,
	)
	if out, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("unable to create release object: %v: %s", err, string(out))
	}
	return nil
}
