package migrate

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg/goods"
	"github.com/replicatedhq/embedded-cluster/pkg/manager"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
)

func RunInstallJobs(ctx context.Context, licenseID string, licenseEndpoint string, versionLabel string) error {
	return nil
}

// InstallAndStartManager installs and starts the manager service on the host.
func InstallAndStartManager(ctx context.Context, licenseID string, licenseEndpoint string, versionLabel string) error {
	binPath := runtimeconfig.PathToEmbeddedClusterBinary("manager")

	// TODO: airgap
	err := manager.DownloadBinaryOnline(ctx, binPath, licenseID, licenseEndpoint, versionLabel)
	if err != nil {
		return fmt.Errorf("download manager binary: %w", err)
	}

	err = manager.Install(ctx, goods.NewMaterializer(), logrus.Infof)
	if err != nil {
		return fmt.Errorf("install manager: %w", err)
	}

	return nil
}
