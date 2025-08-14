package install

import (
	"errors"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

// detectRegistrySettings determines registry settings based on airgap status and service CIDR
func (c *InstallController) detectRegistrySettings(license *kotsv1beta1.License) (*types.RegistrySettings, error) {
	settings := &types.RegistrySettings{}

	// Check if this is an airgap install
	isAirgap := c.airgapBundle != ""
	if !isAirgap {
		return settings, nil // Return empty settings for online installs
	}

	settings.HasLocalRegistry = true

	// Runtime config is required for airgap installs to determine registry host
	if c.rc == nil {
		return nil, errors.New("runtime config is required for airgap registry detection")
	}

	// Get registry host using deterministic function
	serviceCIDR := c.rc.ServiceCIDR()
	registryIP, err := registry.GetRegistryClusterIP(serviceCIDR)
	if err != nil {
		return nil, fmt.Errorf("failed to determine registry cluster IP: %w", err)
	}
	settings.Host = fmt.Sprintf("%s:5000", registryIP)

	settings.Namespace = license.Spec.AppSlug
	settings.ImagePullSecretName = fmt.Sprintf("%s-registry", license.Spec.AppSlug)

	// Set full address
	if settings.Namespace != "" {
		settings.Address = fmt.Sprintf("%s/%s", settings.Host, settings.Namespace)
	} else {
		settings.Address = settings.Host
	}

	return settings, nil
}
