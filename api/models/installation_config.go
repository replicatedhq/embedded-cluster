package models

import (
	"errors"
	"fmt"
	"os"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
)

type InstallationConfig struct {
	AdminConsolePassword    string `json:"adminConsolePassword"`
	AdminConsolePort        int    `json:"adminConsolePort"`
	DataDirectory           string `json:"dataDirectory"`
	HostCABundlePath        string `json:"hostCaBundlePath"`
	LocalArtifactMirrorPort int    `json:"localArtifactMirrorPort"`
	HTTPProxy               string `json:"httpProxy"`
	HTTPSProxy              string `json:"httpsProxy"`
	NoProxy                 string `json:"noProxy"`
	NetworkInterface        string `json:"networkInterface"`
	PodCIDR                 string `json:"podCidr"`
	ServiceCIDR             string `json:"serviceCidr"`
	GlobalCIDR              string `json:"globalCidr"`
	EndUserConfigOverrides  string `json:"endUserConfigOverrides"`
}

func (c *InstallationConfig) Validate() error {
	if err := c.validateCIDR(); err != nil {
		return err
	}

	if err := c.validateNetworkInterface(); err != nil {
		return err
	}

	if err := c.validatePorts(); err != nil {
		return err
	}

	return nil
}

func (c *InstallationConfig) validateCIDR() error {
	if c.PodCIDR != "" && c.ServiceCIDR == "" {
		return errors.New("serviceCidr is required when podCidr is set")
	}

	if c.ServiceCIDR != "" && c.PodCIDR == "" {
		return errors.New("podCidr is required when serviceCidr is set")
	}

	if c.GlobalCIDR != "" {
		if c.PodCIDR != "" || c.ServiceCIDR != "" {
			podCIDR, serviceCIDR, err := netutils.SplitNetworkCIDR(c.GlobalCIDR)
			if err != nil {
				return fmt.Errorf("globalCidr: %w", err)
			}

			if podCIDR != c.PodCIDR {
				return fmt.Errorf("podCidr does not match globalCIDR")
			}

			if serviceCIDR != c.ServiceCIDR {
				return fmt.Errorf("serviceCidr does not match globalCIDR")
			}
		}

		if err := netutils.ValidateCIDR(c.GlobalCIDR, 16, true); err != nil {
			return fmt.Errorf("globalCidr: %w", err)
		}
	}

	return nil
}

func (c *InstallationConfig) validateNetworkInterface() error {
	if c.NetworkInterface == "" {
		return nil
	}

	// TODO: validate the network interface exists and is up and not loopback

	return nil
}

func (c *InstallationConfig) validatePorts() error {
	lamPort := c.LocalArtifactMirrorPort
	acPort := c.AdminConsolePort

	if lamPort != 0 && acPort != 0 {
		if lamPort == acPort {
			return fmt.Errorf("localArtifactMirrorPort and adminConsolePort cannot be equal")
		}
	}

	return nil
}

func (c *InstallationConfig) SetDefaults() error {
	if c.AdminConsolePort == 0 {
		c.AdminConsolePort = ecv1beta1.DefaultAdminConsolePort
	}

	if c.DataDirectory == "" {
		c.DataDirectory = ecv1beta1.DefaultDataDir
	}

	// if a host CA bundle path was not provided, attempt to discover it
	if c.HostCABundlePath == "" {
		hostCABundlePath, err := findHostCABundle()
		if err != nil {
			return fmt.Errorf("unable to find host CA bundle: %w", err)
		}
		c.HostCABundlePath = hostCABundlePath
	}

	if c.LocalArtifactMirrorPort == 0 {
		c.LocalArtifactMirrorPort = ecv1beta1.DefaultLocalArtifactMirrorPort
	}

	// if a network interface was not provided, attempt to discover it
	if c.NetworkInterface == "" {
		autoInterface, err := netutils.DetermineBestNetworkInterface()
		if err == nil {
			c.NetworkInterface = autoInterface
		}
	}

	if err := c.setCIDRDefaults(); err != nil {
		return fmt.Errorf("unable to set cidr defaults: %w", err)
	}

	c.setProxyDefaults()

	return nil
}

func (c *InstallationConfig) setProxyDefaults() {
	if c.HTTPProxy == "" {
		if envValue := os.Getenv("http_proxy"); envValue != "" {
			// logger.Debug("got http_proxy from http_proxy env var")
			c.HTTPProxy = envValue
		} else if envValue := os.Getenv("HTTP_PROXY"); envValue != "" {
			// logger.Debug("got http_proxy from HTTP_PROXY env var")
			c.HTTPProxy = envValue
		}
	}
	if c.HTTPSProxy == "" {
		if envValue := os.Getenv("https_proxy"); envValue != "" {
			// logger.Debug("got https_proxy from https_proxy env var")
			c.HTTPSProxy = envValue
		} else if envValue := os.Getenv("HTTPS_PROXY"); envValue != "" {
			// logger.Debug("got https_proxy from HTTPS_PROXY env var")
			c.HTTPSProxy = envValue
		}
	}
	if c.NoProxy == "" {
		if envValue := os.Getenv("no_proxy"); envValue != "" {
			// logger.Debug("got no_proxy from no_proxy env var")
			c.NoProxy = envValue
		} else if envValue := os.Getenv("NO_PROXY"); envValue != "" {
			// logger.Debug("got no_proxy from NO_PROXY env var")
			c.NoProxy = envValue
		}
	}
}

func (c *InstallationConfig) setCIDRDefaults() error {
	if c.PodCIDR == "" && c.ServiceCIDR == "" {
		if c.GlobalCIDR == "" {
			c.GlobalCIDR = ecv1beta1.DefaultNetworkCIDR
		}

		podCIDR, serviceCIDR, err := netutils.SplitNetworkCIDR(c.GlobalCIDR)
		if err != nil {
			return fmt.Errorf("split network cidr: %w", err)
		}
		c.PodCIDR = podCIDR
		c.ServiceCIDR = serviceCIDR

		return nil
	}

	if c.PodCIDR == "" {
		c.PodCIDR = k0sv1beta1.DefaultNetwork().PodCIDR
	}

	if c.ServiceCIDR == "" {
		c.ServiceCIDR = k0sv1beta1.DefaultNetwork().ServiceCIDR
	}

	return nil
}

// findHostCABundle locates the system CA certificate bundle on the host.
// It first checks the SSL_CERT_FILE environment variable, then searches
// common file paths used by various Linux distributions.
//
// The search follows the same order as the Go standard library's crypto/x509 package.
//
// Returns the path to the first found CA certificate bundle and nil error on success.
// Returns an empty string and error if SSL_CERT_FILE is set but inaccessible
// or if no CA certificate bundle is found.
func findHostCABundle() (string, error) {
	// First check if SSL_CERT_FILE environment variable is set
	if envFile := os.Getenv("SSL_CERT_FILE"); envFile != "" {
		if _, err := os.Stat(envFile); err != nil {
			return "", fmt.Errorf("SSL_CERT_FILE set to %s but file cannot be accessed: %w", envFile, err)
		}
		return envFile, nil
	}

	// From https://github.com/golang/go/blob/go1.24.3/src/crypto/x509/root_linux.go
	certFiles := []string{
		"/etc/ssl/certs/ca-certificates.crt",                // Debian/Ubuntu/Gentoo etc.
		"/etc/pki/tls/certs/ca-bundle.crt",                  // Fedora/RHEL 6
		"/etc/ssl/ca-bundle.pem",                            // OpenSUSE
		"/etc/pki/tls/cacert.pem",                           // OpenELEC
		"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem", // CentOS/RHEL 7
		"/etc/ssl/cert.pem",                                 // Alpine Linux
	}

	// Check each file in the order of preference returning the first found
	for _, file := range certFiles {
		// Ignore all errors to replicate the behavior of the Go standard library
		// https://github.com/golang/go/blob/go1.24.3/src/crypto/x509/root_unix.go#L47-L81
		if _, err := os.Stat(file); err == nil {
			return file, nil
		}
	}

	return "", errors.New("no CA certificate file found")
}
