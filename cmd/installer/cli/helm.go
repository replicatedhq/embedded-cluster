package cli

import (
	"context"
	"log/slog"
	"os"

	// Import to initialize client auth plugins.
	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	helmcmd "helm.sh/helm/v4/pkg/cmd"
	"helm.sh/helm/v4/pkg/kube"
)

// Copied from https://github.com/helm/helm/blob/main/cmd/helm/helm.go
// Should be close to identical.

func HelmCmd(ctx context.Context) *cobra.Command {
	// Setting the name of the app for managedFields in the Kubernetes client.
	// It is set here to the full name of "helm" so that renaming of helm to
	// another name (e.g., helm2 or helm3) does not change the name of the
	// manager as picked up by the automated name detection.
	kube.ManagedFieldsManager = "helm"

	cmd, err := helmcmd.NewRootCmd(os.Stdout, os.Args[1:], helmcmd.SetupLogging)
	if err != nil {
		slog.Warn("command failed", slog.Any("error", err))
		os.Exit(1)
	}

	return cmd
}
