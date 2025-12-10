package cli

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func AdminConsoleUpdateTLSCmd(ctx context.Context, name string) *cobra.Command {
	var tlsCertPath string
	var tlsKeyPath string
	var hostname string

	cmd := &cobra.Command{
		Use:   "update-tls",
		Short: fmt.Sprintf("Update the TLS certificate and key for the %s Admin Console", name),
		Long: fmt.Sprintf(`Update the TLS certificate and key used by the %s Admin Console.

This command updates the kotsadm-tls secret, or creates it if it does not exist.
Pods using this secret are expected to watch for changes and automatically reload
the TLS configuration. This provides a secure alternative to the acceptAnonymousUploads
workflow.

The --hostname flag is optional and only affects the display URL shown
to users. It does not affect TLS certificate validation.`, name),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if !dryrun.Enabled() && os.Getuid() != 0 {
				return fmt.Errorf("update-tls command must be run as root")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdateTLS(cmd.Context(), tlsCertPath, tlsKeyPath, hostname)
		},
	}

	cmd.Flags().StringVar(&tlsCertPath, "tls-cert", "", "Path to the TLS certificate file (required)")
	cmd.Flags().StringVar(&tlsKeyPath, "tls-key", "", "Path to the TLS key file (required)")
	cmd.Flags().StringVar(&hostname, "hostname", "", "Hostname for display in URLs (optional, does not affect TLS validation)")

	cmd.MarkFlagRequired("tls-cert")
	cmd.MarkFlagRequired("tls-key")

	return cmd
}

func runUpdateTLS(ctx context.Context, tlsCertPath, tlsKeyPath, hostname string) error {
	certBytes, keyBytes, err := readAndValidateTLSFiles(tlsCertPath, tlsKeyPath)
	if err != nil {
		return err
	}

	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	namespace, err := runtimeconfig.KotsadmNamespace(ctx, kcli)
	if err != nil {
		return fmt.Errorf("failed to get kotsadm namespace: %w", err)
	}

	loading := spinner.Start()
	loading.Infof("Updating TLS certificate")

	// Update the TLS secret. No pod restart is needed because pods using this secret
	// are expected to watch for changes and automatically reload the TLS configuration.
	if err := updateTLSSecret(ctx, kcli, namespace, certBytes, keyBytes, hostname); err != nil {
		loading.ErrorClosef("Failed to update TLS secret")
		return err
	}

	loading.Closef("TLS certificate updated successfully")
	logrus.Info("")
	logrus.Info("The Admin Console is now using the new TLS certificate.")

	return nil
}

func readAndValidateTLSFiles(certPath, keyPath string) ([]byte, []byte, error) {
	certBytes, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read TLS certificate file: %w", err)
	}

	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read TLS key file: %w", err)
	}

	if _, err := tls.X509KeyPair(certBytes, keyBytes); err != nil {
		return nil, nil, fmt.Errorf("invalid TLS certificate/key pair: %w", err)
	}

	return certBytes, keyBytes, nil
}

func updateTLSSecret(ctx context.Context, kcli client.Client, namespace string, certBytes, keyBytes []byte, hostname string) error {
	secret := &corev1.Secret{}
	err := kcli.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      adminconsole.TLSSecretName(),
	}, secret)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("getting kotsadm-tls secret: %w", err)
	}

	if apierrors.IsNotFound(err) {
		secret = adminconsole.NewTLSSecret(namespace, certBytes, keyBytes, hostname)
		if err := kcli.Create(ctx, secret); err != nil {
			return fmt.Errorf("creating kotsadm-tls secret: %w", err)
		}
		return nil
	}

	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data["tls.crt"] = certBytes
	secret.Data["tls.key"] = keyBytes

	if hostname != "" {
		if secret.StringData == nil {
			secret.StringData = make(map[string]string)
		}
		secret.StringData["hostname"] = hostname
	}

	if err := kcli.Update(ctx, secret); err != nil {
		return fmt.Errorf("updating kotsadm-tls secret: %w", err)
	}

	return nil
}
