package airgap

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/types"
	kotscli "github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
)

// Process processes the airgap bundle
func (m *airgapManager) Process(ctx context.Context, registrySettings *types.RegistrySettings) error {
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
		RegistryAddress:  registrySettings.LocalRegistryAddress,
		RegistryUsername: registrySettings.LocalRegistryUsername,
		RegistryPassword: registrySettings.LocalRegistryPassword,
		ClusterID:        m.clusterID,
		Stdout:           m.newLogWriter(),
	})
	if err != nil {
		return fmt.Errorf("push images to registry: %w", err)
	}

	return nil
}
