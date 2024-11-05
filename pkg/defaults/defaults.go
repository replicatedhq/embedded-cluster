// Package defaults holds default values for the embedded-cluster binary. For sake of
// keeping everything simple this packages exits(1) if some error occurs as
// these should not happen in the first place.
package defaults

import (
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gosimple/slug"
	"github.com/sirupsen/logrus"
)

// Holds the default no proxy values.
var DefaultNoProxy = []string{"localhost", "127.0.0.1", ".cluster.local", ".svc"}

const ProxyRegistryAddress = "proxy.replicated.com"

const KotsadmNamespace = "kotsadm"
const SeaweedFSNamespace = "seaweedfs"
const RegistryNamespace = "registry"
const VeleroNamespace = "velero"

// BinaryName returns the binary name, this is useful for places where we
// need to present the name of the binary to the user (the name may vary if
// the binary is renamed). We make sure the name does not contain invalid
// characters for a filename.
func BinaryName() string {
	exe, err := os.Executable()
	if err != nil {
		logrus.Fatalf("unable to get executable path: %s", err)
	}
	base := filepath.Base(exe)
	return slug.Make(base)
}

// EmbeddedClusterLogsSubDir returns the path to the directory where embedded-cluster logs
// are stored.
func EmbeddedClusterLogsSubDir() string {
	path := "/var/log/embedded-cluster"
	if err := os.MkdirAll(path, 0755); err != nil {
		logrus.Fatalf("unable to create embedded-cluster logs dir: %s", err)
	}
	return path
}

// PathToLog returns the full path to a log file. This function does not check
// if the file exists.
func PathToLog(name string) string {
	return filepath.Join(EmbeddedClusterLogsSubDir(), name)
}

// K0sBinaryPath returns the path to the k0s binary when it is installed on the node. This
// does not return the binary just after we materilized it but the path we want it to be
// once it is installed.
func K0sBinaryPath() string {
	return "/usr/local/bin/k0s"
}

// TryDiscoverPublicIP tries to discover the public IP of the node by querying
// a list of known providers. If the public IP cannot be discovered, an empty
// string is returned.

func TryDiscoverPublicIP() string {
	// List of providers and their respective metadata URLs
	providers := []struct {
		name string
		fn   func() string
	}{
		{
			name: "gce",
			fn: func() string {
				return makeMetadataRequest(
					"GET",
					"http://169.254.169.254/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip",
					map[string]string{"Metadata-Flavor": "Google"},
				)
			},
		},
		{
			name: "ec2",
			fn: func() string {
				return makeMetadataRequest(
					"GET",
					"http://169.254.169.254/latest/meta-data/public-ipv4",
					nil,
				)
			},
		},
		{
			name: "ec2",
			fn: func() string {
				token := makeMetadataRequest(
					"PUT",
					"http://169.254.169.254/latest/api/token",
					map[string]string{"X-aws-ec2-metadata-token-ttl-seconds": "60"},
				)
				if token == "" {
					return ""
				}
				return makeMetadataRequest(
					"GET",
					"http://169.254.169.254/latest/meta-data/public-ipv4",
					map[string]string{"X-aws-ec2-metadata-token": token},
				)
			},
		},
		{
			name: "azure",
			fn: func() string {
				return makeMetadataRequest(
					"GET",
					"http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/publicIpAddress?api-version=2017-08-01&format=text",
					map[string]string{"Metadata": "true"},
				)
			},
		},
	}

	for _, provider := range providers {
		publicIP := provider.fn()
		if publicIP != "" {
			return publicIP
		}
	}
	return ""
}

func makeMetadataRequest(method string, u string, headers map[string]string) string {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil // no proxy
	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: transport,
	}

	req, _ := http.NewRequest(method, u, nil)
	for k, v := range headers {
		req.Header.Add(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	publicIP := string(bodyBytes)
	if net.ParseIP(publicIP).To4() != nil {
		return publicIP
	}
	return ""
}

// PathToK0sStatusSocket returns the full path to the k0s status socket.
func PathToK0sStatusSocket() string {
	return "/run/k0s/status.sock"
}

// PathToK0sConfig returns the full path to the k0s configuration file.
func PathToK0sConfig() string {
	return "/etc/k0s/k0s.yaml"
}

// PathToK0sContainerdConfig returns the full path to the k0s containerd configuration directory
func PathToK0sContainerdConfig() string {
	return "/etc/k0s/containerd.d/"
}

// PathToECConfig returns the full path to the embedded cluster configuration file.
// This file is used to specify the embedded cluster data directory.
func PathToECConfig() string {
	return "/etc/embedded-cluster/ec.yaml"
}
