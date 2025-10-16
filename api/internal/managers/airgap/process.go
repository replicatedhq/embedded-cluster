package airgap

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/replicatedhq/embedded-cluster/api/types"
	kotscli "github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
)

// Process processes the airgap bundle
func (m *airgapManager) Process(ctx context.Context, registrySettings *types.RegistrySettings) (finalErr error) {
	if err := m.setStatus(types.StateRunning, "Processing air gap bundle"); err != nil {
		return fmt.Errorf("set status: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v: %s", r, string(debug.Stack()))
		}
		if finalErr != nil {
			if err := m.setStatus(types.StateFailed, finalErr.Error()); err != nil {
				m.logger.WithError(err).Error("set failed status")
			}
		} else {
			if err := m.setStatus(types.StateSucceeded, "Airgap bundle processed"); err != nil {
				m.logger.WithError(err).Error("set succeeded status")
			}
		}
	}()

	if err := m.process(ctx, registrySettings); err != nil {
		return err
	}

	return nil
}

func (m *airgapManager) process(ctx context.Context, registrySettings *types.RegistrySettings) error {
	// Skip if not an airgap installation
	if m.airgapBundle == "" {
		return nil
	}

	// Validate registry settings
	if registrySettings == nil {
		return fmt.Errorf("registry settings are required for airgap installations")
	}

	m.addLogs("Processing air gap bundle")

	err := kotscli.PushImages(kotscli.PushImagesOptions{
		AirgapBundle:     m.airgapBundle,
		RegistryAddress:  registrySettings.Address,
		RegistryUsername: registrySettings.Username,
		RegistryPassword: registrySettings.Password,
		ClusterID:        m.clusterID,
		Stdout:           m.newLogWriter(),
	})
	if err != nil {
		return fmt.Errorf("push images to registry: %w", err)
	}

	return nil
}
