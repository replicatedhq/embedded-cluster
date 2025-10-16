package cli

import (
	"context"
	"os"

	// Import to initialize client auth plugins.
	"github.com/sirupsen/logrus"
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

	// Strip off "<app-name> helm" from the args and pass the rest of the args to HelmCmd
	if len(os.Args) < 2 {
		logrus.Fatalf("insufficient arguments for helm command: %v", os.Args)
	}
	osArgs := os.Args[2:]

	cmd, err := helmcmd.NewRootCmd(os.Stdout, osArgs, helmcmd.SetupLogging)
	if err != nil {
		logrus.Infof("command failed: %v", err)
		os.Exit(1)
	}

	// Hide the helm subcommand from the help output.
	// We wrap it inside a script that's accessible in the shell.
	cmd.Hidden = true
	return cmd
}
