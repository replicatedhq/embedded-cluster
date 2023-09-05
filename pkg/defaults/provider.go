package defaults

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gosimple/slug"
)

const (
	k0sBinsSubDirDarwin = "Library/Caches/k0sctl/k0s/linux/amd64"
	k0sBinsSubDirLinux  = ".cache/k0sctl/k0s/linux/amd64"
)

// NewProvider returns a new DefaultsProvider using the provided base dir.
// Base is the base directory inside which all the other directories are
// created.
func NewProvider(base string) *DefaultsProvider {
	obj := &DefaultsProvider{Base: base}
	obj.Init()
	return obj
}

// DefaultsProvider is an entity that provides default values used during
// HelmVM installation.
type DefaultsProvider struct {
	Base string
}

// Init makes sure all the necessary directory exists on the system.
func (d *DefaultsProvider) Init() {
	if err := os.MkdirAll(d.K0sctlBinsSubDir(), 0755); err != nil {
		panic(fmt.Errorf("unable to create basedir: %w", err))
	}
	if err := os.MkdirAll(d.ConfigSubDir(), 0755); err != nil {
		panic(fmt.Errorf("unable to create config dir: %w", err))
	}
	if err := os.MkdirAll(d.HelmVMBinsSubDir(), 0755); err != nil {
		panic(fmt.Errorf("unable to create helmvm bin dir: %w", err))
	}
	if err := os.MkdirAll(d.HelmVMLogsSubDir(), 0755); err != nil {
		panic(fmt.Errorf("unable to create helmvm logs dir: %w", err))
	}
}

// BinaryName returns the binary name, this is useful for places where we
// need to present the name of the binary to the user (the name may vary if
// the binary is renamed). We make sure the name does not contain invalid
// characters for a filename.
func (d *DefaultsProvider) BinaryName() string {
	exe, err := os.Executable()
	if err != nil {
		panic(err)
	}
	base := filepath.Base(exe)
	return slug.Make(base)
}

// HelmVMLogsSubDir returns the path to the directory where helmvm logs are
// stored. This is a subdirectory of the user's home directory.
func (d *DefaultsProvider) HelmVMLogsSubDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	hidden := fmt.Sprintf(".%s", BinaryName())
	return filepath.Join(home, hidden, "logs")
}

// PathToLog returns the full path to a log file. This function does not check
// if the file exists.
func (d *DefaultsProvider) PathToLog(name string) string {
	return filepath.Join(d.HelmVMLogsSubDir(), name)
}

// K0sctlBinsSubDir returns the path to the directory where k0sctl binaries
// are stored. This is a subdirectory of the user's home directory. Follows
// the k0sctl directory convention.
func (d *DefaultsProvider) K0sctlBinsSubDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(d.Base, home, k0sBinsSubDirDarwin)
	}
	return filepath.Join(d.Base, home, k0sBinsSubDirLinux)
}

// HelmVMBinsSubDir returns the path to the directory where helmvm binaries
// are stored. This is a subdirectory of the user's home directory.
func (d *DefaultsProvider) HelmVMBinsSubDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	hidden := fmt.Sprintf(".%s", d.BinaryName())
	return filepath.Join(d.Base, home, hidden, "bin")
}

// K0sctlApplyLogPath returns the path to the k0sctl apply log file.
func (d *DefaultsProvider) K0sctlApplyLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return filepath.Join(d.Base, home, ".cache", "k0sctl", "k0sctl.log")
}

// SSHKeyPath returns the path to the SSH managed by helmvm installation.
func (d *DefaultsProvider) SSHKeyPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return filepath.Join(d.Base, home, ".ssh", "helmvm_rsa")
}

// SSHAuthorizedKeysPath returns the path to the authorized_hosts file.
func (d *DefaultsProvider) SSHAuthorizedKeysPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return filepath.Join(d.Base, home, ".ssh", "authorized_keys")
}

// ConfigSubDir returns the path to the directory where k0sctl configuration
// files are stored. This is a subdirectory of the user's home directory.
func (d *DefaultsProvider) ConfigSubDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	hidden := fmt.Sprintf(".%s", d.BinaryName())
	return filepath.Join(d.Base, home, hidden, "etc")
}

// K0sBinaryPath returns the path to the k0s binary.
func (d *DefaultsProvider) K0sBinaryPath() string {
	return d.PathToK0sctlBinary(fmt.Sprintf("k0s-%s", K0sVersion))
}

// PathToK0sctlBinary is an utility function that returns the full path to
// a materialized binary that belongs to k0sctl. This function does not check
// if the file exists.
func (d *DefaultsProvider) PathToK0sctlBinary(name string) string {
	return filepath.Join(d.K0sctlBinsSubDir(), name)
}

// PathToHelmVMBinary is an utility function that returns the full path to a
// materialized binary that belongs to helmvm (do not confuse with binaries
// belonging to k0sctl). This function does not check if the file exists.
func (d *DefaultsProvider) PathToHelmVMBinary(name string) string {
	return filepath.Join(d.HelmVMBinsSubDir(), name)
}

// PathToConfig returns the full path to a configuration file. This function
// does not check if the file exists.
func (d *DefaultsProvider) PathToConfig(name string) string {
	return filepath.Join(d.ConfigSubDir(), name)
}

// FileNameForImage returns an appropriate .tar name for a given image.
// e.g. quay.io/test/test:v1 would return quay.io-test-test-v1.tar.
func (d *DefaultsProvider) FileNameForImage(img string) string {
	prefix := strings.ReplaceAll(img, "/", "-")
	prefix = strings.ReplaceAll(prefix, ":", "-")
	prefix = strings.ReplaceAll(prefix, "@", "-")
	return fmt.Sprintf("%s.tar", prefix)
}

// PreferredNodeIPAddress returns the ip address the node uses when reaching
// the internet. This is useful when the node has multiple interfaces and we
// want to bind to one of the interfaces.
func (d *DefaultsProvider) PreferredNodeIPAddress() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		return "", fmt.Errorf("unable to get local IP: %w", err)
	}
	defer conn.Close()
	addr := conn.LocalAddr().(*net.UDPAddr)
	return addr.IP.String(), nil
}

// DecentralizedInstall returns true if the cluster installation has been
// executed in a decentralized way (installing the first node then generating
// a join token and installing the others).
func (d *DefaultsProvider) DecentralizedInstall() bool {
	fpath := d.PathToConfig(".decentralized")
	_, err := os.Stat(fpath)
	return err == nil
}

// SetInstallAsDecentralized sets the decentralized install flag inside the
// configuration directory.
func (d *DefaultsProvider) SetInstallAsDecentralized() error {
	fpath := d.PathToConfig(".decentralized")
	fp, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("unable to set installation mode: %w", err)
	}
	defer fp.Close()
	return nil
}
