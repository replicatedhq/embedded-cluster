package migrate

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg/goods"
	"github.com/replicatedhq/embedded-cluster/pkg/manager"
	"github.com/sirupsen/logrus"
)

// Migrate stops and removes the operator service and installs and starts the manager service.
func Migrate(ctx context.Context) error {
	err := installAndStartManager(ctx)
	if err != nil {
		return fmt.Errorf("install and start manager: %w", err)
	}

	return nil
}

func installAndStartManager(ctx context.Context) error {
	materializer := goods.NewMaterializer()

	logrus.Infof("Manager unit name is %q", manager.UnitName())

	logrus.Infof("Writing manager systemd unit file")
	err := manager.WriteSystemdUnitFile(materializer)
	if err != nil {
		return fmt.Errorf("write manager systemd unit file: %w", err)
	}
	logrus.Infof("Successfully wrote manager systemd unit file")

	logrus.Infof("Writing manager drop-in file")
	if err := manager.WriteDropInFile(); err != nil {
		return fmt.Errorf("write manager drop-in file: %w", err)
	}
	logrus.Infof("Successfully wrote manager drop-in file")

	logrus.Infof("Enabling and starting manager service")
	if err := manager.EnableAndStart(ctx); err != nil {
		return fmt.Errorf("enable and start manager service: %w", err)
	}
	logrus.Infof("Successfully enabled and started manager service")

	return nil
}
