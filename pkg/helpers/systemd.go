package helpers

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers/systemd"
)

// IsSystemdServiceActive checks if a systemd service is active or not.
func (h *Helpers) IsSystemdServiceActive(ctx context.Context, svcname string) (bool, error) {
	return systemd.IsActive(ctx, svcname)
}
