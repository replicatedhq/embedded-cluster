package domains

import (
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
)

const DefaultReplicatedAppDomain = "replicated.app"
const DefaultProxyRegistryDomain = "proxy.replicated.com"
const DefaultReplicatedRegistryDomain = "registry.replicated.com"

// GetDomains returns the domains for the embedded cluster. The first priority is the domains configured within the provided config spec.
// The second priority is the domains configured within the channel release. If neither is configured, the default domains are returned.
func GetDomains(cfgspec *ecv1beta1.ConfigSpec, rel *release.ChannelRelease) ecv1beta1.Domains {
	replicatedAppDomain := DefaultReplicatedAppDomain
	proxyRegistryDomain := DefaultProxyRegistryDomain
	replicatedRegistryDomain := DefaultReplicatedRegistryDomain

	// get defaults from channel release if available
	if rel != nil {
		if rel.DefaultDomains.ReplicatedAppDomain != "" {
			replicatedAppDomain = rel.DefaultDomains.ReplicatedAppDomain
		}
		if rel.DefaultDomains.ProxyRegistryDomain != "" {
			proxyRegistryDomain = rel.DefaultDomains.ProxyRegistryDomain
		}
		if rel.DefaultDomains.ReplicatedRegistryDomain != "" {
			replicatedRegistryDomain = rel.DefaultDomains.ReplicatedRegistryDomain
		}
	}

	// get overrides from config spec if available
	if cfgspec != nil {
		if cfgspec.Domains.ReplicatedAppDomain != "" {
			replicatedAppDomain = cfgspec.Domains.ReplicatedAppDomain
		}
		if cfgspec.Domains.ProxyRegistryDomain != "" {
			proxyRegistryDomain = cfgspec.Domains.ProxyRegistryDomain
		}
		if cfgspec.Domains.ReplicatedRegistryDomain != "" {
			replicatedRegistryDomain = cfgspec.Domains.ReplicatedRegistryDomain
		}
	}

	return ecv1beta1.Domains{
		ReplicatedAppDomain:      replicatedAppDomain,
		ProxyRegistryDomain:      proxyRegistryDomain,
		ReplicatedRegistryDomain: replicatedRegistryDomain,
	}
}
