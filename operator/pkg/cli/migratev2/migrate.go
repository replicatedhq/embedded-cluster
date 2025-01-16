package migratev2

import (
	"context"
	"fmt"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LogFunc can be used as an argument to Run to log messages.
type LogFunc func(string, ...any)

// Run runs the v1 to v2 migration. It installs the manager service on all nodes, copies the
// installations to configmaps, enables the v2 admin console, and finally removes the operator
// chart.
func Run(
	ctx context.Context, logf LogFunc, cli client.Client, helmCLI helm.Client,
	in *ecv1beta1.Installation,
	licenseSecret string, appSlug string, appVersionLabel string,
) error {
	err := runManagerInstallJobsAndWait(ctx, logf, cli, in, licenseSecret, appSlug, appVersionLabel)
	if err != nil {
		return fmt.Errorf("run manager install jobs: %w", err)
	}

	err = copyInstallationsToConfigMaps(ctx, logf, cli)
	if err != nil {
		return fmt.Errorf("copy installations to config maps: %w", err)
	}

	// We must first uninstall the operator to ensure that it does not reconcile and revert our
	// changes.
	err = uninstallOperator(ctx, logf, cli, helmCLI)
	if err != nil {
		return fmt.Errorf("uninstall operator: %w", err)
	}

	err = enableV2AdminConsole(ctx, logf, cli, in)
	if err != nil {
		return fmt.Errorf("enable v2 admin console: %w", err)
	}

	err = cleanupV1(ctx, logf, cli)
	if err != nil {
		return fmt.Errorf("cleanup v1: %w", err)
	}

	return nil
}
