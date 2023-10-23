package defaults

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gosimple/slug"
	"github.com/sirupsen/logrus"
)

const (
	k0sBinsSubDirDarwin = "Library/Caches/k0sctl/k0s/linux/amd64"
	k0sBinsSubDirLinux  = ".cache/k0sctl/k0s/linux/amd64"
)

// NewProvider returns a new Provider using the provided base dir.
// Base is the base directory inside which all the other directories are
// created.
func NewProvider(base string) *Provider {
	obj := &Provider{Base: base}
	obj.Init()
	return obj
}

// Provider is an entity that provides default values used during
// HelmVM installation.
type Provider struct {
	Base string
}

// Init makes sure all the necessary directory exists on the system.
func (d *Provider) Init() {
	if err := os.MkdirAll(d.K0sctlBinsSubDir(), 0755); err != nil {
		logrus.Fatalf("unable to create basedir: %s", err)
	}
	if err := os.MkdirAll(d.ConfigSubDir(), 0755); err != nil {
		logrus.Fatalf("unable to create config dir: %s", err)
	}
	if err := os.MkdirAll(d.HelmVMBinsSubDir(), 0755); err != nil {
		logrus.Fatalf("unable to create embedded-cluster bin dir: %s", err)
	}
	if err := os.MkdirAll(d.HelmVMLogsSubDir(), 0755); err != nil {
		panic(fmt.Errorf("unable to create embedded-cluster logs dir: %w", err))
	}
	if err := os.MkdirAll(d.SSHConfigSubDir(), 0700); err != nil {
		logrus.Fatalf("unable to create ssh config dir: %s", err)
	}
	if err := os.MkdirAll(d.HelmChartSubDir(), 0755); err != nil {
		logrus.Fatalf("unable to create helm chart dir: %s", err)
	}
}

// home returns the user's home dir.
func (d *Provider) home() string {
	home, err := os.UserHomeDir()
	if err != nil {
		logrus.Fatalf("unable to get user home dir: %s", err)
	}
	return home
}

// config returns the user's config dir.
func (d *Provider) config() string {
	home, err := os.UserHomeDir()
	if err != nil {
		logrus.Fatalf("unable to get user home dir: %s", err)
	}

	// use the XDG_CONFIG_HOME environment variable if set
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		return xdgConfigHome
	}

	// otherwise, default to $HOME/.config on linux
	if runtime.GOOS == "linux" {
		return filepath.Join(home, ".config")
	}

	return home
}

// state returns the user's state dir.
func (d *Provider) state() string {
	home, err := os.UserHomeDir()
	if err != nil {
		logrus.Fatalf("unable to get user home dir: %s", err)
	}

	// use the XDG_STATE_HOME environment variable if set
	if xdgStateHome := os.Getenv("XDG_STATE_HOME"); xdgStateHome != "" {
		return xdgStateHome
	}

	// otherwise, default to $HOME/.local/state on linux
	if runtime.GOOS == "linux" {
		return filepath.Join(home, ".local", "state")
	}

	return home
}

// BinaryName returns the binary name, this is useful for places where we
// need to present the name of the binary to the user (the name may vary if
// the binary is renamed). We make sure the name does not contain invalid
// characters for a filename.
func (d *Provider) BinaryName() string {
	exe, err := os.Executable()
	if err != nil {
		logrus.Fatalf("unable to get executable path: %s", err)
	}
	base := filepath.Base(exe)
	return slug.Make(base)
}

// HelmVMLogsSubDir returns the path to the directory where embedded-cluster logs are
// stored. This is a subdirectory of the user's home directory.
func (d *Provider) HelmVMLogsSubDir() string {
	hidden := fmt.Sprintf(".%s", d.BinaryName())
	return filepath.Join(d.Base, d.state(), hidden, "logs")
}

// PathToLog returns the full path to a log file. This function does not check
// if the file exists.
func (d *Provider) PathToLog(name string) string {
	return filepath.Join(d.HelmVMLogsSubDir(), name)
}

// K0sctlBinsSubDir returns the path to the directory where k0sctl binaries
// are stored. This is a subdirectory of the user's home directory. Follows
// the k0sctl directory convention.
func (d *Provider) K0sctlBinsSubDir() string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(d.Base, d.home(), k0sBinsSubDirDarwin)
	}
	return filepath.Join(d.Base, d.home(), k0sBinsSubDirLinux)
}

// HelmChartSubDir returns the path to the directory where helm charts are stored
func (d *Provider) HelmChartSubDir() string {
	hidden := fmt.Sprintf(".%s", d.BinaryName())
	return filepath.Join(d.Base, d.home(), hidden, "charts")
}

// HelmVMBinsSubDir returns the path to the directory where embedded-cluster binaries
// are stored. This is a subdirectory of the user's home directory.
func (d *Provider) HelmVMBinsSubDir() string {
	hidden := fmt.Sprintf(".%s", d.BinaryName())
	return filepath.Join(d.Base, d.config(), hidden, "bin")
}

// K0sctlApplyLogPath returns the path to the k0sctl apply log file.
func (d *Provider) K0sctlApplyLogPath() string {
	return filepath.Join(d.Base, d.home(), ".cache", "k0sctl", "k0sctl.log")
}

// SSHKeyPath returns the path to the SSH managed by embedded-cluster installation.
func (d *Provider) SSHKeyPath() string {
	return filepath.Join(d.Base, d.home(), ".ssh", "embedded-cluster_rsa")
}

// SSHAuthorizedKeysPath returns the path to the authorized_hosts file.
func (d *Provider) SSHAuthorizedKeysPath() string {
	return filepath.Join(d.SSHConfigSubDir(), "authorized_keys")
}

// SSHConfigSubDir returns the path to the directory where SSH configuration
// files are stored. This is a subdirectory of the user's home directory.
func (d *Provider) SSHConfigSubDir() string {
	return filepath.Join(d.Base, d.home(), ".ssh")
}

// ConfigSubDir returns the path to the directory where k0sctl configuration
// files are stored. This is a subdirectory of the user's home directory.
// TODO update
func (d *Provider) ConfigSubDir() string {
	hidden := fmt.Sprintf(".%s", d.BinaryName())
	return filepath.Join(d.Base, d.config(), hidden, "etc")
}

// K0sBinaryPath returns the path to the k0s binary.
func (d *Provider) K0sBinaryPath() string {
	return d.PathToK0sctlBinary(fmt.Sprintf("k0s-%s", K0sVersion))
}

// PathToK0sctlBinary is an utility function that returns the full path to
// a materialized binary that belongs to k0sctl. This function does not check
// if the file exists.
func (d *Provider) PathToK0sctlBinary(name string) string {
	return filepath.Join(d.K0sctlBinsSubDir(), name)
}

// PathToHelmVMBinary is an utility function that returns the full path to a
// materialized binary that belongs to embedded-cluster (do not confuse with binaries
// belonging to k0sctl). This function does not check if the file exists.
func (d *Provider) PathToHelmVMBinary(name string) string {
	return filepath.Join(d.HelmVMBinsSubDir(), name)
}

// PathToHelmChart returns the path to a materialized helm chart.
func (d *Provider) PathToHelmChart(name string, version string) string {
	return filepath.Join(d.HelmChartSubDir(), name+"-"+version+".tgz")
}

// PathToConfig returns the full path to a configuration file. This function
// does not check if the file exists.
func (d *Provider) PathToConfig(name string) string {
	return filepath.Join(d.ConfigSubDir(), name)
}

// FileNameForImage returns an appropriate .tar name for a given image.
// e.g. quay.io/test/test:v1 would return quay.io-test-test-v1.tar.
func (d *Provider) FileNameForImage(img string) string {
	prefix := strings.ReplaceAll(img, "/", "-")
	prefix = strings.ReplaceAll(prefix, ":", "-")
	prefix = strings.ReplaceAll(prefix, "@", "-")
	return fmt.Sprintf("%s.tar", prefix)
}

// PreferredNodeIPAddress returns the ip address the node uses when reaching
// the internet. This is useful when the node has multiple interfaces and we
// want to bind to one of the interfaces.
func (d *Provider) PreferredNodeIPAddress() (string, error) {
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
func (d *Provider) DecentralizedInstall() bool {
	fpath := d.PathToConfig(".decentralized")
	_, err := os.Stat(fpath)
	return err == nil
}

// SetInstallAsDecentralized sets the decentralized install flag inside the
// configuration directory.
func (d *Provider) SetInstallAsDecentralized() error {
	fpath := d.PathToConfig(".decentralized")
	fp, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("unable to set installation mode: %w", err)
	}
	defer fp.Close()
	return nil
}

// IsUpgrade determines if we are upgrading a cluster judging by the existence
// or not of a kubeconfig file in the configuration directory.
func (d *Provider) IsUpgrade() bool {
	fpath := d.PathToConfig("kubeconfig")
	_, err := os.Stat(fpath)
	return err == nil
}

// TryDiscoverPublicIP tries to discover the public IP of the node by querying
// a list of known providers. If the public IP cannot be discovered, an empty
// string is returned.
func (d *Provider) TryDiscoverPublicIP() string {
	// List of providers and their respective metadata URLs
	providers := []struct {
		name    string
		url     string
		headers map[string]string
	}{
		{"gce", "http://169.254.169.254/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip", map[string]string{"Metadata-Flavor": "Google"}},
		{"ec2", "http://169.254.169.254/latest/meta-data/public-ipv4", nil},
		{"azure", "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/publicIpAddress?api-version=2017-08-01&format=text", map[string]string{"Metadata": "true"}},
	}

	for _, provider := range providers {
		client := &http.Client{
			Timeout: 5 * time.Second,
		}
		req, _ := http.NewRequest("GET", provider.url, nil)
		for k, v := range provider.headers {
			req.Header.Add(k, v)
		}
		resp, err := client.Do(req)
		if err != nil {
			return ""
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			publicIP := string(bodyBytes)
			if isValidIPv4(publicIP) {
				return publicIP
			} else {
				return ""
			}
		}
	}
	return ""
}

func isValidIPv4(ip string) bool {
	return net.ParseIP(ip).To4() != nil
}
