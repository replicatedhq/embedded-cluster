package migratev2

import (
	"context"
	"fmt"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ecNamespace = "embedded-cluster"
)

type LogFunc func(string, ...any)

func Run(
	ctx context.Context, logf LogFunc, cli client.Client,
	in *ecv1beta1.Installation,
	licenseSecret string, appVersionLabel string,
) error {
	err := runManagerInstallJobsAndWait(ctx, logf, cli, in, licenseSecret, appVersionLabel)
	if err != nil {
		return fmt.Errorf("run manager install jobs: %w", err)
	}

	err = copyInstallationsToConfigMaps(ctx, logf, cli)
	if err != nil {
		return fmt.Errorf("copy installations to config maps: %w", err)
	}

	// Configure KOTS to Enable EC V2

	err = cleanupV1(ctx, logf, cli)
	if err != nil {
		return fmt.Errorf("cleanup v1: %w", err)
	}

	return nil
}
