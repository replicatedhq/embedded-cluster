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
	return manager.Install(ctx, goods.NewMaterializer(), logrus.Infof)
}
