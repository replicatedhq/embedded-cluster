package install

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

// detectRegistrySettings determines registry settings based on airgap status and service CIDR
func (c *InstallController) detectRegistrySettings(license *kotsv1beta1.License) *types.RegistrySettings {
	settings := &types.RegistrySettings{}

	// Check if this is an airgap install
	isAirgap := c.airgapBundle != ""
	if !isAirgap {
		return settings // Return empty settings for online installs
	}

	settings.HasLocalRegistry = true

	// Get registry host using deterministic function
	if c.rc != nil {
		serviceCIDR := c.rc.ServiceCIDR()
		if registryIP, err := registry.GetRegistryClusterIP(serviceCIDR); err == nil {
			settings.Host = fmt.Sprintf("%s:5000", registryIP)
		}
	}

	// Set namespace and other fields based on license
	if license != nil && license.Spec.AppSlug != "" {
		settings.Namespace = license.Spec.AppSlug
		settings.ImagePullSecretName = fmt.Sprintf("%s-registry", license.Spec.AppSlug)
	}

	// Set full address
	if settings.Namespace != "" {
		settings.Address = fmt.Sprintf("%s/%s", settings.Host, settings.Namespace)
	} else {
		settings.Address = settings.Host
	}

	return settings
}
