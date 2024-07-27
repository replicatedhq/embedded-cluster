package goods

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/replicatedhq/embedded-cluster/sdk/defaults"
)

// Materializer is an entity capable of materialize (write to disk) embedded assets.
type Materializer struct {
	def *defaults.Provider
}

// NewMaterializer returns a new entity capable of materialize (write to disk) embedded
// assets. Other operations on embedded assets are also available.
func NewMaterializer(basedir string) *Materializer {
	return &Materializer{def: defaults.NewProvider(basedir)}
}

// InternalBinary materializes an internal binary from inside internal/bins directory
// and writes it to a tmp file. It returns the path to the materialized binary.
// The binary should be deleted after it is used.
// This is used for binaries that are not meant to be exposed to the user.
func (m *Materializer) InternalBinary(name string) (string, error) {
	srcpath := fmt.Sprintf("internal/bins/%s", name)
	srcfile, err := internalBinfs.ReadFile(srcpath)
	if err != nil {
		return "", fmt.Errorf("unable to read asset: %w", err)
	}
	dstpath, err := os.CreateTemp("", fmt.Sprintf("embedded-cluster-%s-bin-", name))
	if err != nil {
		return "", fmt.Errorf("unable to create temp file: %w", err)
	}
	defer dstpath.Close()
	if _, err := dstpath.Write(srcfile); err != nil {
		return "", fmt.Errorf("unable to write file: %w", err)
	}
	if err := dstpath.Chmod(0755); err != nil {
		return "", fmt.Errorf("unable to set executable permissions: %w", err)
	}
	return dstpath.Name(), nil
}

// LocalArtifactMirrorUnitFile writes to disk the local-artifact-mirror systemd unit file.
func (m *Materializer) LocalArtifactMirrorUnitFile() error {
	content, err := systemdfs.ReadFile("systemd/local-artifact-mirror.service")
	if err != nil {
		return fmt.Errorf("unable to open unit file: %w", err)
	}
	dstpath := "/etc/systemd/system/local-artifact-mirror.service"
	if err := os.WriteFile(dstpath, content, 0644); err != nil {
		return fmt.Errorf("unable to write file: %w", err)
	}
	return nil
}

// CalicoNetworkManagerConfig materializes a configuration file for the network manager. This
// configuration file instructs the network manager to ignore any interface being managed by
// the calico network cni.
func (m *Materializer) CalicoNetworkManagerConfig() error {
	content, err := systemdfs.ReadFile("systemd/calico-network-manager.conf")
	if err != nil {
		return fmt.Errorf("unable to open network manager config file: %w", err)
	}
	dstpath := "/etc/NetworkManager/conf.d/embedded-cluster.conf"
	if err := os.WriteFile(dstpath, content, 0644); err != nil {
		return fmt.Errorf("unable to write file: %w", err)
	}
	return nil
}

// Materialize writes to disk all embedded assets.
func (m *Materializer) Materialize() error {
	if err := m.Binaries(); err != nil {
		return fmt.Errorf("unable to materialize embedded binaries: %w", err)
	}
	if err := m.SupportFiles(); err != nil {
		return fmt.Errorf("unable to materialize embedded support files: %w", err)
	}
	return nil
}

// SupportFiles materializes files under the support directory.
func (m *Materializer) SupportFiles() error {
	entries, err := supportfs.ReadDir("support")
	if err != nil {
		return fmt.Errorf("unable to read embedded-cluster support dir: %w", err)
	}
	for _, entry := range entries {
		srcpath := fmt.Sprintf("support/%s", entry.Name())
		srcfile, err := supportfs.ReadFile(srcpath)
		if err != nil {
			return fmt.Errorf("unable to read asset: %w", err)
		}
		dstpath := m.def.PathToEmbeddedClusterSupportFile(entry.Name())
		if err := os.WriteFile(dstpath, srcfile, 0700); err != nil {
			return fmt.Errorf("unable to write file: %w", err)
		}
	}
	return nil
}

// Binaries materializes all binary files from inside bins directory. If the file already
// exists a copy of it is made first before overwriting it, this is done because we can't
// overwrite a running binary. Copies are removed. This function also creates a copy of
// this binary into the PathToEmbeddedClusterBinary() directory.
func (m *Materializer) Binaries() error {
	entries, err := binfs.ReadDir("bins")
	if err != nil {
		return fmt.Errorf("unable to read embedded-cluster bins dir: %w", err)
	}

	if err := m.Ourselves(); err != nil {
		return fmt.Errorf("unable to materialize ourselves: %w", err)
	}

	var remove []string
	defer func() {
		for _, f := range remove {
			os.Remove(f)
		}
	}()

	for _, entry := range entries {
		srcpath := fmt.Sprintf("bins/%s", entry.Name())
		srcfile, err := binfs.ReadFile(srcpath)
		if err != nil {
			return fmt.Errorf("unable to read asset: %w", err)
		}

		dstpath := m.def.PathToEmbeddedClusterBinary(entry.Name())
		if _, err := os.Stat(dstpath); err == nil {
			tmp := fmt.Sprintf("%s.bkp", dstpath)
			if err := os.Rename(dstpath, tmp); err != nil {
				return fmt.Errorf("unable to rename %s to %s: %w", dstpath, tmp, err)
			}
			remove = append(remove, tmp)
		}

		if err := os.WriteFile(dstpath, srcfile, 0755); err != nil {
			return fmt.Errorf("unable to write file: %w", err)
		}
	}

	return nil
}

// K0sBinarySHA256 returns the SHA256 checksum of the embedded k0s binary.
func (m *Materializer) K0sBinarySHA256() (string, error) {
	fp, err := binfs.Open("bins/k0s")
	if err != nil {
		return "", fmt.Errorf("unable to open embedded k0s binary: %w", err)
	}
	defer fp.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, fp); err != nil {
		return "", fmt.Errorf("unable to copy embedded k0s binary: %w", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// Ourselves makes a copy of the embedded-cluster binary into the PathToEmbeddedClusterBinary() directory.
// We are doing this copy for three reasons: 1. We make sure we have it in a standard location across all
// installations. 2. We can overwrite it during cluster upgrades. 3. we can serve a copy of the binary
// through the local-artifact-mirror daemon.
func (m *Materializer) Ourselves() error {
	srcpath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("unable to get our own executable path: %w", err)
	}

	dstpath := m.def.PathToEmbeddedClusterBinary(m.def.BinaryName())
	if srcpath == dstpath {
		return nil
	}

	if _, err := os.Stat(dstpath); err == nil {
		tmp := fmt.Sprintf("%s.bkp", dstpath)
		if err := os.Rename(dstpath, tmp); err != nil {
			return fmt.Errorf("unable to rename %s to %s: %w", dstpath, tmp, err)
		}
		defer os.Remove(tmp)
	}

	src, err := os.Open(srcpath)
	if err != nil {
		return fmt.Errorf("unable to open source file: %w", err)
	}
	defer src.Close()

	opts := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	dst, err := os.OpenFile(dstpath, opts, 0755)
	if err != nil {
		return fmt.Errorf("unable to open destination file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("unable to write file: %w", err)
	}
	return nil
}
