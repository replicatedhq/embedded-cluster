package customization

import (
	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster-operator/api/v1beta1"
	"github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

// Provider abstracts the embedded Kots Application Release data provider. It is
// used mostly for testing. Can be useful later down the road for other providers
// of the embedded data (we may also decide to change the way we embed the data).
type Provider interface {
	HostPreflights() (*v1beta2.HostPreflightSpec, error)
	License() (*v1beta1.License, error)
	Application() ([]byte, error)
	EmbeddedClusterConfig() (*embeddedclusterv1beta1.Config, error)
	ChannelRelease() (*ChannelRelease, error)
}

// DefaultProvider is the global customization provider, it exists to make it easier
// to mock out the customization provider in tests. It is also specially useful for
// replacing the way we manage the embedded data later down the road.
var DefaultProvider Provider = &AdminConsole{}

// GetHostPreflights returns a list of HostPreflight specs that are found in the
// binary. These are part of the embedded Kots Application Release.
func GetHostPreflights() (*v1beta2.HostPreflightSpec, error) {
	return DefaultProvider.HostPreflights()
}

// GetLicense reads the kots license from the embedded Kots Application Release. If
// no license is found, returns nil and no error.
func GetLicense() (*v1beta1.License, error) {
	return DefaultProvider.License()
}

// GetApplication reads and returns the kots application embedded as part of the
// release. If no application is found, returns nil and no error. This function does
// not unmarshal the application yaml.
func GetApplication() ([]byte, error) {
	return DefaultProvider.Application()
}

// GetEmbeddedClusterConfig reads the embedded cluster config from the embedded Kots
// Application Release.
func GetEmbeddedClusterConfig() (*embeddedclusterv1beta1.Config, error) {
	return DefaultProvider.EmbeddedClusterConfig()
}

// ChannelRelease reads the embedded channel release object. If no channel release
// is found, returns nil and no error.
func GetChannelRelease() (*ChannelRelease, error) {
	return DefaultProvider.ChannelRelease()
}
