package migrate

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg/goods"
	"github.com/replicatedhq/embedded-cluster/pkg/manager"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
)

// Migrate stops and removes the operator service and installs and starts the manager service.
func Migrate(ctx context.Context, licenseID string, licenseEndpoint string, versionLabel string) error {
	err := installAndStartManager(ctx, licenseID, licenseEndpoint, versionLabel)
	if err != nil {
		return fmt.Errorf("install and start manager: %w", err)
	}

	return nil
}

func installAndStartManager(ctx context.Context, licenseID string, licenseEndpoint string, versionLabel string) error {
	binPath := runtimeconfig.PathToEmbeddedClusterBinary("manager")

	// TODO: airgap
	err := downloadManagerBinaryOnline(ctx, licenseID, licenseEndpoint, versionLabel, binPath)
	if err != nil {
		return fmt.Errorf("download manager binary: %w", err)
	}

	err = manager.Install(ctx, goods.NewMaterializer(), logrus.Infof)
	if err != nil {
		return fmt.Errorf("install manager: %w", err)
	}

	return nil
}
