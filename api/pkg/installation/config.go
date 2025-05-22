package installation

import (
	"errors"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
)

func ConfigValidate(config *types.InstallationConfig) error {
	var ve *types.APIError

	if err := configValidateGlobalCIDR(config); err != nil {
		ve = types.AppendFieldError(ve, "globalCidr", err)
	}

	if err := configValidatePodCIDR(config); err != nil {
		ve = types.AppendFieldError(ve, "podCidr", err)
	}

	if err := configValidateServiceCIDR(config); err != nil {
		ve = types.AppendFieldError(ve, "serviceCidr", err)
	}

	if err := configValidateNetworkInterface(config); err != nil {
		ve = types.AppendFieldError(ve, "networkInterface", err)
	}

	if err := configValidateAdminConsolePort(config); err != nil {
		ve = types.AppendFieldError(ve, "adminConsolePort", err)
	}

	if err := configValidateLocalArtifactMirrorPort(config); err != nil {
		ve = types.AppendFieldError(ve, "localArtifactMirrorPort", err)
	}

	return ve.ErrorOrNil()
}

func configValidateGlobalCIDR(config *types.InstallationConfig) error {
	if config.GlobalCIDR == "" {
		return nil
	}

	if err := netutils.ValidateCIDR(config.GlobalCIDR, 16, true); err != nil {
		return err
	}

	podCIDR, serviceCIDR, err := netutils.SplitNetworkCIDR(config.GlobalCIDR)
	if err != nil {
		return fmt.Errorf("split globalCidr: %w", err)
	}
	if config.PodCIDR != "" && podCIDR != config.PodCIDR {
		return errors.New("podCidr does not match globalCIDR")
	}
	if config.ServiceCIDR != "" && serviceCIDR != config.ServiceCIDR {
		return errors.New("serviceCidr does not match globalCIDR")
	}

	return nil
}

func configValidatePodCIDR(config *types.InstallationConfig) error {
	if config.ServiceCIDR != "" && config.PodCIDR == "" {
		return errors.New("podCidr is required when serviceCidr is set")
	}
	return nil
}

func configValidateServiceCIDR(config *types.InstallationConfig) error {
	if config.PodCIDR != "" && config.ServiceCIDR == "" {
		return errors.New("serviceCidr is required when podCidr is set")
	}
	return nil
}

func configValidateNetworkInterface(config *types.InstallationConfig) *types.APIError {
	if config.NetworkInterface == "" {
		return nil
	}

	// TODO: validate the network interface exists and is up and not loopback

	return nil
}

func configValidateAdminConsolePort(config *types.InstallationConfig) *types.APIError {
	if config.AdminConsolePort == 0 {
		return nil
	}

	lamPort := config.LocalArtifactMirrorPort
	if lamPort == 0 {
		lamPort = ecv1beta1.DefaultLocalArtifactMirrorPort
	}

	if config.AdminConsolePort == lamPort {
		return types.NewBadRequestError(errors.New("adminConsolePort and localArtifactMirrorPort cannot be equal"))
	}

	return nil
}

func configValidateLocalArtifactMirrorPort(config *types.InstallationConfig) *types.APIError {
	if config.LocalArtifactMirrorPort == 0 {
		return nil
	}

	acPort := config.AdminConsolePort
	if acPort == 0 {
		acPort = ecv1beta1.DefaultAdminConsolePort
	}

	if config.LocalArtifactMirrorPort == acPort {
		return types.NewBadRequestError(errors.New("adminConsolePort and localArtifactMirrorPort cannot be equal"))
	}

	return nil
}

func ConfigSetDefaults(config *types.InstallationConfig) error {
	if config.AdminConsolePort == 0 {
		config.AdminConsolePort = ecv1beta1.DefaultAdminConsolePort
	}

	if config.DataDirectory == "" {
		config.DataDirectory = ecv1beta1.DefaultDataDir
	}

	// if a host CA bundle path was not provided, attempt to discover it
	if config.HostCABundlePath == "" {
		hostCABundlePath, err := findHostCABundle()
		if err != nil {
			return fmt.Errorf("unable to find host CA bundle: %w", err)
		}
		config.HostCABundlePath = hostCABundlePath
	}

	if config.LocalArtifactMirrorPort == 0 {
		config.LocalArtifactMirrorPort = ecv1beta1.DefaultLocalArtifactMirrorPort
	}

	// if a network interface was not provided, attempt to discover it
	if config.NetworkInterface == "" {
		autoInterface, err := netutils.DetermineBestNetworkInterface()
		if err == nil {
			config.NetworkInterface = autoInterface
		}
	}

	if err := configSetCIDRDefaults(config); err != nil {
		return fmt.Errorf("unable to set cidr defaults: %w", err)
	}

	configSetProxyDefaults(config)

	return nil
}

func configSetProxyDefaults(config *types.InstallationConfig) {
	if config.HTTPProxy == "" {
		if envValue := os.Getenv("http_proxy"); envValue != "" {
			// logger.Debug("got http_proxy from http_proxy env var")
			config.HTTPProxy = envValue
		} else if envValue := os.Getenv("HTTP_PROXY"); envValue != "" {
			// logger.Debug("got http_proxy from HTTP_PROXY env var")
			config.HTTPProxy = envValue
		}
	}
	if config.HTTPSProxy == "" {
		if envValue := os.Getenv("https_proxy"); envValue != "" {
			// logger.Debug("got https_proxy from https_proxy env var")
			config.HTTPSProxy = envValue
		} else if envValue := os.Getenv("HTTPS_PROXY"); envValue != "" {
			// logger.Debug("got https_proxy from HTTPS_PROXY env var")
			config.HTTPSProxy = envValue
		}
	}
	if config.NoProxy == "" {
		if envValue := os.Getenv("no_proxy"); envValue != "" {
			// logger.Debug("got no_proxy from no_proxy env var")
			config.NoProxy = envValue
		} else if envValue := os.Getenv("NO_PROXY"); envValue != "" {
			// logger.Debug("got no_proxy from NO_PROXY env var")
			config.NoProxy = envValue
		}
	}
}

func configSetCIDRDefaults(config *types.InstallationConfig) error {
	if config.PodCIDR == "" && config.ServiceCIDR == "" {
		if config.GlobalCIDR == "" {
			config.GlobalCIDR = ecv1beta1.DefaultNetworkCIDR
		}

		podCIDR, serviceCIDR, err := netutils.SplitNetworkCIDR(config.GlobalCIDR)
		if err != nil {
			return fmt.Errorf("split network cidr: %w", err)
		}
		config.PodCIDR = podCIDR
		config.ServiceCIDR = serviceCIDR

		return nil
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
