package cli

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/goods"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/replicatedhq/embedded-cluster/pkg-new/tlsutils"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/configutils"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// hydrateInstallCmdFlags modifies flags in-place to resolve final values
// This handles cmd.Changed(), env vars, auto-detection, and validation
// Call this AFTER cobra has parsed flags but BEFORE using them
func hydrateInstallCmdFlags(cmd *cobra.Command, flags *InstallCmdFlags) error {
	// Target defaulting (if not V3)
	if !isV3Enabled() {
		flags.target = "linux"
	}

	// Target validation
	if flags.target != "linux" && flags.target != "kubernetes" {
		return fmt.Errorf(`invalid --target (must be one of: "linux", "kubernetes")`)
	}

	// If only one of cert or key is provided, return an error
	if (flags.tlsCertFile != "" && flags.tlsKeyFile == "") || (flags.tlsCertFile == "" && flags.tlsKeyFile != "") {
		return fmt.Errorf("both --tls-cert and --tls-key must be provided together")
	}

	// Skip host preflights from env var (if flag not explicitly set)
	if !cmd.Flags().Changed("skip-host-preflights") {
		if os.Getenv("SKIP_HOST_PREFLIGHTS") == "1" || os.Getenv("SKIP_HOST_PREFLIGHTS") == "true" {
			flags.skipHostPreflights = true
		}
	}

	// Network interface auto-detection (if not provided)
	if flags.networkInterface == "" && flags.target == "linux" {
		autoInterface, err := newconfig.DetermineBestNetworkInterface()
		if err == nil {
			flags.networkInterface = autoInterface
		}
		// If error, leave empty and validation will catch it later
	}

	// Port conflict validations
	if flags.managerPort != 0 && flags.adminConsolePort != 0 {
		if flags.managerPort == flags.adminConsolePort {
			return fmt.Errorf("manager port cannot be the same as admin console port")
		}
	}

	if flags.localArtifactMirrorPort != 0 && flags.adminConsolePort != 0 {
		if flags.localArtifactMirrorPort == flags.adminConsolePort {
			return fmt.Errorf("local artifact mirror port cannot be the same as admin console port")
		}
	}

	// Admin console password
	if cmd.Flags().Lookup("admin-console-password") != nil {
		if err := ensureAdminConsolePassword(flags); err != nil {
			return err
		}
	}

	// CIDR configuration
	cidrCfg, err := cidrConfigFromCmd(cmd)
	if err != nil {
		return err
	}
	flags.cidrConfig = cidrCfg

	// Proxy configuration
	proxy, err := proxyConfigFromCmd(cmd, flags.networkInterface, flags.cidrConfig, flags.assumeYes)
	if err != nil {
		return err
	}
	flags.proxySpec = proxy

	return nil
}

// buildInstallDerivedConfig takes hydrated flags and computes all derived values
// This function has side effects: file I/O, crypto generation
func buildInstallDerivedConfig(flags *InstallCmdFlags) (*InstallDerivedConfig, error) {
	derived := &InstallDerivedConfig{
		clusterID:               uuid.New().String(),
		enableManagerExperience: isV3Enabled(),
	}

	// === File I/O Operations ===

	// License file reading
	if flags.licenseFile != "" {
		b, err := os.ReadFile(flags.licenseFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read license file: %w", err)
		}
		derived.licenseBytes = b

		l, err := helpers.ParseLicense(flags.licenseFile)
		if err != nil {
			return nil, fmt.Errorf("failed to parse license file: %w", err)
		}
		derived.license = l
	}

	// Config values validation
	if flags.configValues != "" {
		err := configutils.ValidateKotsConfigValues(flags.configValues)
		if err != nil {
			return nil, fmt.Errorf("config values file is not valid: %w", err)
		}
	}

	// Airgap detection and metadata
	derived.isAirgap = flags.airgapBundle != ""
	if flags.airgapBundle != "" {
		metadata, err := airgap.AirgapMetadataFromPath(flags.airgapBundle)
		if err != nil {
			return nil, fmt.Errorf("failed to get airgap info: %w", err)
		}
		derived.airgapMetadata = metadata
	}

	// Embedded assets size
	size, err := goods.SizeOfEmbeddedAssets()
	if err != nil {
		return nil, fmt.Errorf("failed to get size of embedded files: %w", err)
	}
	derived.embeddedAssetsSize = size

	// End user config (overrides file)
	eucfg, err := helpers.ParseEndUserConfig(flags.overrides)
	if err != nil {
		return nil, fmt.Errorf("process overrides file: %w", err)
	}
	derived.endUserConfig = eucfg

	// TLS Certificate Processing
	if err := processTLSConfig(flags, derived); err != nil {
		return nil, fmt.Errorf("process TLS config: %w", err)
	}

	return derived, nil
}

func ensureAdminConsolePassword(flags *InstallCmdFlags) error {
	if flags.adminConsolePassword == "" {
		// no password was provided
		if flags.assumeYes {
			logrus.Infof("\nThe Admin Console password is set to %q.", "password")
			flags.adminConsolePassword = "password"
		} else {
			logrus.Info("")
			maxTries := 3
			for i := 0; i < maxTries; i++ {
				promptA, err := prompts.New().Password(fmt.Sprintf("Set the Admin Console password (minimum %d characters):", minAdminPasswordLength))
				if err != nil {
					return fmt.Errorf("failed to get password: %w", err)
				}

				promptB, err := prompts.New().Password("Confirm the Admin Console password:")
				if err != nil {
					return fmt.Errorf("failed to get password confirmation: %w", err)
				}

				if validateAdminConsolePassword(promptA, promptB) {
					flags.adminConsolePassword = promptA
					return nil
				}
			}
			return NewErrorNothingElseToAdd(errors.New("password is not valid"))
		}
	}

	if !validateAdminConsolePassword(flags.adminConsolePassword, flags.adminConsolePassword) {
		return NewErrorNothingElseToAdd(errors.New("password is not valid"))
	}

	return nil
}

func proxyConfigFromCmd(cmd *cobra.Command, networkInterface string, cidrCfg *newconfig.CIDRConfig, assumeYes bool) (*ecv1beta1.ProxySpec, error) {
	proxy, err := parseProxyFlags(cmd, networkInterface, cidrCfg)
	if err != nil {
		return nil, err
	}

	if err := verifyProxyConfig(proxy, prompts.New(), assumeYes); err != nil {
		return nil, err
	}

	return proxy, nil
}

func cidrConfigFromCmd(cmd *cobra.Command) (*newconfig.CIDRConfig, error) {
	if err := validateCIDRFlags(cmd); err != nil {
		return nil, err
	}

	// parse the various cidr flags to make sure we have exactly what we want
	cidrCfg, err := getCIDRConfig(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to determine pod and service CIDRs: %w", err)
	}

	return cidrCfg, nil
}

func processTLSConfig(flags *InstallCmdFlags, derived *InstallDerivedConfig) error {
	// If both cert and key are provided, load them
	if flags.tlsCertFile != "" && flags.tlsKeyFile != "" {
		certBytes, err := os.ReadFile(flags.tlsCertFile)
		if err != nil {
			return fmt.Errorf("failed to read TLS certificate: %w", err)
		}
		keyBytes, err := os.ReadFile(flags.tlsKeyFile)
		if err != nil {
			return fmt.Errorf("failed to read TLS key: %w", err)
		}

		cert, err := tls.X509KeyPair(certBytes, keyBytes)
		if err != nil {
			return fmt.Errorf("failed to parse TLS certificate: %w", err)
		}

		derived.tlsCert = cert
		derived.tlsCertBytes = certBytes
		derived.tlsKeyBytes = keyBytes
	} else if derived.enableManagerExperience {
		// For manager experience, generate self-signed certificate if none provided
		logrus.Warn("\nNo certificate files provided. A self-signed certificate will be used, and your browser will show a security warning.")
		logrus.Info("To use your own certificate, provide both --tls-key and --tls-cert flags.")

		if !flags.assumeYes {
			logrus.Info("") // newline so the prompt is separated from the warning
			confirmed, err := prompts.New().Confirm("Do you want to continue with a self-signed certificate?", false)
			if err != nil {
				return fmt.Errorf("failed to get confirmation: %w", err)
			}
			if !confirmed {
				logrus.Infof("\nInstallation cancelled. Please run the command again with the --tls-key and --tls-cert flags.\n")
				return fmt.Errorf("installation cancelled by user")
			}
		}

		// Get all IP addresses for the certificate
		ipAddresses, err := netutils.ListAllValidIPAddresses()
		if err != nil {
			return fmt.Errorf("failed to list all valid IP addresses: %w", err)
		}

		// Determine the namespace for the certificate
		kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(context.Background(), nil)
		if err != nil {
			return fmt.Errorf("get kotsadm namespace: %w", err)
		}

		// Generate self-signed certificate
		cert, certData, keyData, err := tlsutils.GenerateCertificate(flags.hostname, ipAddresses, kotsadmNamespace)
		if err != nil {
			return fmt.Errorf("generate tls certificate: %w", err)
		}
		derived.tlsCert = cert
		derived.tlsCertBytes = certData
		derived.tlsKeyBytes = keyData
	}

	return nil
}
