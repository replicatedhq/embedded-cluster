package defaults

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gosimple/slug"
	"github.com/sirupsen/logrus"
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
// EmbeddedCluster installation.
type Provider struct {
	Base string
}

// Init makes sure all the necessary directory exists on the system.
func (d *Provider) Init() {
	if err := os.MkdirAll(d.EmbeddedClusterConfigSubDir(), 0755); err != nil {
		logrus.Fatalf("unable to create config dir: %s", err)
	}
	if err := os.MkdirAll(d.EmbeddedClusterBinsSubDir(), 0755); err != nil {
		logrus.Fatalf("unable to create embedded-cluster bin dir: %s", err)
	}
	if err := os.MkdirAll(d.EmbeddedClusterLogsSubDir(), 0755); err != nil {
		panic(fmt.Errorf("unable to create embedded-cluster logs dir: %w", err))
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

// EmbeddedClusterLogsSubDir returns the path to the directory where embedded-cluster logs
// are stored.
func (d *Provider) EmbeddedClusterLogsSubDir() string {
	return filepath.Join(d.Base, d.config(), d.BinaryName(), "logs")
}

// PathToLog returns the full path to a log file. This function does not check
// if the file exists.
func (d *Provider) PathToLog(name string) string {
	return filepath.Join(d.EmbeddedClusterLogsSubDir(), name)
}

// EmbeddedClusterBinsSubDir returns the path to the directory where embedded-cluster binaries
// are stored.
func (d *Provider) EmbeddedClusterBinsSubDir() string {
	return filepath.Join(d.Base, d.config(), d.BinaryName(), "bin")
}

// EmbeddedClusterConfigSubDir returns the path to the directory where configuration files are
// stored.
func (d *Provider) EmbeddedClusterConfigSubDir() string {
	return filepath.Join(d.Base, d.config(), d.BinaryName(), "etc")
}

// EmbeddedClusterHomeDirectory returns the parent directory. Inside this parent directory we
// store all the embedded-cluster related files.
func (d *Provider) EmbeddedClusterHomeDirectory() string {
	return filepath.Join(d.Base, d.config(), d.BinaryName())
}

// K0sBinaryPath returns the path to the k0s binary when it is installed on the node. This
// does not return the binary just after we materilized it but the path we want it to be
// once it is installed.
func (d *Provider) K0sBinaryPath() string {
	return "/usr/local/bin/k0s"
}

// PathToEmbeddedClusterBinary is an utility function that returns the full path to a
// materialized binary that belongs to embedded-cluster. This function does not check
// if the file exists.
func (d *Provider) PathToEmbeddedClusterBinary(name string) string {
	return filepath.Join(d.EmbeddedClusterBinsSubDir(), name)
}

// PathToConfig returns the full path to a configuration file. This function
// does not check if the file exists.
func (d *Provider) PathToConfig(name string) string {
	return filepath.Join(d.EmbeddedClusterConfigSubDir(), name)
}

func (d *Provider) PathToKubeconfig() string {
	return filepath.Join("~/.kube/config")
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
			if net.ParseIP(publicIP).To4() != nil {
				return publicIP
			} else {
				return ""
			}
		}
	}
	return ""
}

// PathToK0sStatusSocket returns the full path to the k0s status socket.
func (d *Provider) PathToK0sStatusSocket() string {
	return "/run/k0s/status.sock"
}

// PathToK0sConfig returns the full path to the k0s configuration file.
func (d *Provider) PathToK0sConfig() string {
	return "/etc/k0s/k0s.yaml"
}
