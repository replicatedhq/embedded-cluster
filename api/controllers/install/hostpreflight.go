package install

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/internal/managers/preflight"
	"github.com/replicatedhq/embedded-cluster/api/pkg/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
)

func (c *InstallController) RunHostPreflights(ctx context.Context, opts RunHostPreflightsOptions) error {
	// Get the configured custom domains
	ecDomains := utils.GetDomains(c.releaseData)

	// Calculate airgap storage space requirement (2x uncompressed size for controller nodes)
	var controllerAirgapStorageSpace string
	if c.airgapBundle != "" {
		airgapInfo, err := airgap.AirgapInfoFromPath(c.airgapBundle)
		if err != nil {
			return fmt.Errorf("failed to get airgap info: %w", err)
		}
		controllerAirgapStorageSpace = preflights.CalculateAirgapStorageSpace(airgapInfo.Spec.UncompressedSize, true)
	}

	// Prepare host preflights
	hpf, err := c.hostPreflightManager.PrepareHostPreflights(ctx, c.rc, preflight.PrepareHostPreflightOptions{
		ReplicatedAppURL:             netutils.MaybeAddHTTPS(ecDomains.ReplicatedAppDomain),
		ProxyRegistryURL:             netutils.MaybeAddHTTPS(ecDomains.ProxyRegistryDomain),
		HostPreflightSpec:            c.releaseData.HostPreflights,
		EmbeddedClusterConfig:        c.releaseData.EmbeddedClusterConfig,
		IsAirgap:                     c.airgapBundle != "",
		IsUI:                         opts.IsUI,
		ControllerAirgapStorageSpace: controllerAirgapStorageSpace,
	})
	if err != nil {
		return fmt.Errorf("failed to prepare host preflights: %w", err)
	}

	// Run host preflights
	return c.hostPreflightManager.RunHostPreflights(ctx, c.rc, preflight.RunHostPreflightOptions{
		HostPreflightSpec: hpf,
	})
}

func (c *InstallController) GetHostPreflightStatus(ctx context.Context) (types.Status, error) {
	return c.hostPreflightManager.GetHostPreflightStatus(ctx)
}

func (c *InstallController) GetHostPreflightOutput(ctx context.Context) (*types.HostPreflightsOutput, error) {
	return c.hostPreflightManager.GetHostPreflightOutput(ctx)
}

func (c *InstallController) GetHostPreflightTitles(ctx context.Context) ([]string, error) {
	return c.hostPreflightManager.GetHostPreflightTitles(ctx)
}
