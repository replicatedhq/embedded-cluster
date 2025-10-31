package cli

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/goods"
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

// Hop: buildInstallFlags maps cobra command flags to install flags
func buildInstallFlags(cmd *cobra.Command, flags *installFlags) error {
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

	// CIDR configuration
	cidrCfg, err := cidrConfigFromCmd(cmd)
	if err != nil {
		return err
	}
	flags.cidrConfig = cidrCfg

	// Proxy configuration
	proxy, err := parseProxyFlags(cmd, flags.networkInterface, flags.cidrConfig)
	if err != nil {
		return err
	}
	flags.proxySpec = proxy

	return nil
}

// Hop: buildInstallConfig builds the install config from install flags
func buildInstallConfig(flags *installFlags) (*installConfig, error) {
	installCfg := &installConfig{
		clusterID:               uuid.New().String(),
		enableManagerExperience: isV3Enabled(),
	}

	// License file reading
	if flags.licenseFile != "" {
		b, err := os.ReadFile(flags.licenseFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read license file: %w", err)
		}
		installCfg.licenseBytes = b

		l, err := helpers.ParseLicense(flags.licenseFile)
		if err != nil {
			return nil, fmt.Errorf("failed to parse license file: %w", err)
		}
		installCfg.license = l
	}

	// Config values validation
	if flags.configValues != "" {
		err := configutils.ValidateKotsConfigValues(flags.configValues)
		if err != nil {
			return nil, fmt.Errorf("config values file is not valid: %w", err)
		}
	}

	// Airgap detection and metadata
	installCfg.isAirgap = flags.airgapBundle != ""
	if flags.airgapBundle != "" {
		metadata, err := airgap.AirgapMetadataFromPath(flags.airgapBundle)
		if err != nil {
			return nil, fmt.Errorf("failed to get airgap info: %w", err)
		}
		installCfg.airgapMetadata = metadata
	}

	// Embedded assets size
	size, err := goods.SizeOfEmbeddedAssets()
	if err != nil {
		return nil, fmt.Errorf("failed to get size of embedded files: %w", err)
	}
	installCfg.embeddedAssetsSize = size

	// End user config (overrides file)
	eucfg, err := helpers.ParseEndUserConfig(flags.overrides)
	if err != nil {
		return nil, fmt.Errorf("process overrides file: %w", err)
	}
	installCfg.endUserConfig = eucfg

	// TLS Certificate Processing
	if err := processTLSConfig(flags, installCfg); err != nil {
		return nil, fmt.Errorf("process TLS config: %w", err)
	}

	return installCfg, nil
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

func processTLSConfig(flags *installFlags, installCfg *installConfig) error {
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

		installCfg.tlsCert = &cert
		installCfg.tlsCertBytes = certBytes
		installCfg.tlsKeyBytes = keyBytes
	} else if !flags.headless && installCfg.enableManagerExperience {
		// For UI based manager experience, generate self-signed cert if none provided, with user confirmation
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
		installCfg.tlsCert = &cert
		installCfg.tlsCertBytes = certData
		installCfg.tlsKeyBytes = keyData
	}

	return nil
}
