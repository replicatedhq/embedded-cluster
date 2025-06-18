package cli

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/tlsutils"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type InstallLinuxCmdFlags struct {
	managerPort  int
	tlsCertFile  string
	tlsKeyFile   string
	hostname     string
	tlsCert      tls.Certificate
	tlsCertBytes []byte
	tlsKeyBytes  []byte

	installCmdFlags InstallCmdFlags
}

// InstallLinuxCmd returns a cobra command for installing the embedded cluster.
func InstallLinuxCmd(ctx context.Context, name string) *cobra.Command {
	var flags InstallLinuxCmdFlags

	ctx, cancel := context.WithCancel(ctx)
	rc := runtimeconfig.New(nil)

	cmd := &cobra.Command{
		Use:   "linux",
		Short: fmt.Sprintf("linux %s", name),
		PostRun: func(cmd *cobra.Command, args []string) {
			rc.Cleanup()
			cancel() // Cancel context when command completes
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := verifyAndPrompt(ctx, name, flags.installCmdFlags, prompts.New()); err != nil {
				return err
			}

			if err := preRunInstall(cmd, &flags.installCmdFlags, rc); err != nil {
				return err
			}

			if err := runInstallLinux(ctx, flags, rc); err != nil {
				return err
			}

			return nil
		},
	}

	if err := addInstallFlags(cmd, &flags.installCmdFlags); err != nil {
		panic(err)
	}
	if err := addInstallAdminConsoleFlags(cmd, &flags.installCmdFlags); err != nil {
		panic(err)
	}
	if err := addInstallLinuxFlags(cmd, &flags); err != nil {
		panic(err)
	}

	return cmd
}

func addInstallLinuxFlags(cmd *cobra.Command, flags *InstallLinuxCmdFlags) error {
	cmd.Flags().IntVar(&flags.managerPort, "manager-port", ecv1beta1.DefaultManagerPort, "Port on which the Manager will be served")
	cmd.Flags().StringVar(&flags.tlsCertFile, "tls-cert", "", "Path to the TLS certificate file")
	cmd.Flags().StringVar(&flags.tlsKeyFile, "tls-key", "", "Path to the TLS key file")
	cmd.Flags().StringVar(&flags.hostname, "hostname", "", "Hostname to use for TLS configuration")

	return nil
}

func runInstallLinux(ctx context.Context, flags InstallLinuxCmdFlags, rc runtimeconfig.RuntimeConfig) (finalErr error) {
	// this is necessary because the api listens on all interfaces,
	// and we only know the interface to use when the user selects it in the ui
	ipAddresses, err := netutils.ListAllValidIPAddresses()
	if err != nil {
		return fmt.Errorf("unable to list all valid IP addresses: %w", err)
	}

	if flags.tlsCertFile == "" || flags.tlsKeyFile == "" {
		logrus.Warn("\nNo certificate files provided. A self-signed certificate will be used, and your browser will show a security warning.")
		logrus.Info("To use your own certificate, provide both --tls-key and --tls-cert flags.")

		if !flags.installCmdFlags.assumeYes {
			logrus.Info("") // newline so the prompt is separated from the warning
			confirmed, err := prompts.New().Confirm("Do you want to continue with a self-signed certificate?", false)
			if err != nil {
				return fmt.Errorf("failed to get confirmation: %w", err)
			}
			if !confirmed {
				logrus.Infof("\nInstallation cancelled. Please run the command again with the --tls-key and --tls-cert flags.\n")
				return nil
			}
		}
	}

	if flags.tlsCertFile != "" && flags.tlsKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(flags.tlsCertFile, flags.tlsKeyFile)
		if err != nil {
			return fmt.Errorf("load tls certificate: %w", err)
		}
		certData, err := os.ReadFile(flags.tlsCertFile)
		if err != nil {
			return fmt.Errorf("unable to read tls cert file: %w", err)
		}
		keyData, err := os.ReadFile(flags.tlsKeyFile)
		if err != nil {
			return fmt.Errorf("unable to read tls key file: %w", err)
		}
		flags.tlsCert = cert
		flags.tlsCertBytes = certData
		flags.tlsKeyBytes = keyData
	} else {
		cert, certData, keyData, err := tlsutils.GenerateCertificate(flags.hostname, ipAddresses)
		if err != nil {
			return fmt.Errorf("generate tls certificate: %w", err)
		}
		flags.tlsCert = cert
		flags.tlsCertBytes = certData
		flags.tlsKeyBytes = keyData
	}

	eucfg, err := helpers.ParseEndUserConfig(flags.installCmdFlags.overrides)
	if err != nil {
		return fmt.Errorf("process overrides file: %w", err)
	}

	apiConfig := apiConfig{
		// TODO (@salah): implement reporting in api
		// MetricsReporter: installReporter,
		RuntimeConfig: rc,
		Password:      flags.installCmdFlags.adminConsolePassword,
		TLSConfig: apitypes.TLSConfig{
			CertBytes: flags.tlsCertBytes,
			KeyBytes:  flags.tlsKeyBytes,
			Hostname:  flags.hostname,
		},
		ManagerPort:   flags.managerPort,
		LicenseFile:   flags.installCmdFlags.licenseFile,
		AirgapBundle:  flags.installCmdFlags.airgapBundle,
		ConfigValues:  flags.installCmdFlags.configValues,
		ReleaseData:   release.GetReleaseData(),
		EndUserConfig: eucfg,
	}

	if err := startAPI(ctx, flags.tlsCert, apiConfig); err != nil {
		return fmt.Errorf("unable to start api: %w", err)
	}

	// TODO: add app name to this message (e.g., App Name manager)
	logrus.Infof("\nVisit the manager to continue: %s\n", getManagerURL(flags.hostname, flags.managerPort))
	<-ctx.Done()

	return nil
}
