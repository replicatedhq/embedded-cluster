package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg-new/tlsutils"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	rcutil "github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig/util"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func AdminConsoleResetTLSCmd(ctx context.Context, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset-tls",
		Short: fmt.Sprintf("Reset the TLS certificate for the %s Admin Console to the default self-signed certificate", name),
		Long:  fmt.Sprintf("Reset the TLS certificate used by the %s Admin Console to the default self-signed TLS certificate", name),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if !dryrun.Enabled() && os.Getuid() != 0 {
				return fmt.Errorf("reset-tls command must be run as root")
			}
			rc := rcutil.InitBestRuntimeConfig(cmd.Context())
			return rc.SetEnv()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResetTLS(cmd.Context())
		},
	}

	return cmd
}

func runResetTLS(ctx context.Context) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	namespace, err := runtimeconfig.KotsadmNamespace(ctx, kcli)
	if err != nil {
		return fmt.Errorf("failed to get kotsadm namespace: %w", err)
	}

	loading := spinner.Start()
	loading.Infof("Generating new self-signed TLS certificate")

	// Get all IP addresses for the certificate SANs
	ipAddresses, err := netutils.ListAllValidIPAddresses()
	if err != nil {
		loading.ErrorClosef("Failed to list IP addresses")
		return fmt.Errorf("failed to list IP addresses: %w", err)
	}

	// Generate a new self-signed certificate
	_, certBytes, keyBytes, err := tlsutils.GenerateCertificate("", ipAddresses, namespace)
	if err != nil {
		loading.ErrorClosef("Failed to generate certificate")
		return fmt.Errorf("failed to generate self-signed certificate: %w", err)
	}

	loading.Infof("Updating TLS secret")

	// Update the TLS secret. No pod restart is needed because pods using this secret
	// are expected to watch for changes and automatically reload the TLS configuration.
	if err := resetTLSSecret(ctx, kcli, namespace, certBytes, keyBytes); err != nil {
		loading.ErrorClosef("Failed to update TLS secret")
		return err
	}

	loading.Closef("TLS certificate reset successfully")
	logrus.Info("")
	logrus.Info("The Admin Console is now using a new self-signed TLS certificate.")
	logrus.Info("Your browser will show a security warning which you can safely bypass.")

	return nil
}

func resetTLSSecret(ctx context.Context, kcli client.Client, namespace string, certBytes, keyBytes []byte) error {
	secret := &corev1.Secret{}
	err := kcli.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      adminconsole.TLSSecretName(),
	}, secret)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("getting kotsadm-tls secret: %w", err)
	}

	if apierrors.IsNotFound(err) {
		// Create new secret with empty hostname (will use default)
		secret = adminconsole.NewTLSSecret(namespace, certBytes, keyBytes, "")
		if err := kcli.Create(ctx, secret); err != nil {
			return fmt.Errorf("creating kotsadm-tls secret: %w", err)
		}
		return nil
	}

	// Update existing secret, preserving hostname if set
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data["tls.crt"] = certBytes
	secret.Data["tls.key"] = keyBytes

	if err := kcli.Update(ctx, secret); err != nil {
		return fmt.Errorf("updating kotsadm-tls secret: %w", err)
	}

	return nil
}
