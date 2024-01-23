package embed

import (
	"fmt"
	"os"
	"sync"

	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster-operator/api/v1beta1"
	"github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

var (
	mtx         sync.Mutex
	releaseData *ReleaseData
)

// parseReleaseDataFromBinary reads the embedded data from the binary and sets the global
// releaseData variable only once.
func parseReleaseDataFromBinary() error {
	mtx.Lock()
	defer mtx.Unlock()
	if releaseData != nil {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("unable to get executable path: %w", err)
	}
	data, err := ExtractFromBinary(exe)
	if err != nil {
		return fmt.Errorf("failed to extract data from binary: %w", err)
	}
	release, err := NewReleaseDataFrom(data)
	if err != nil {
		return fmt.Errorf("failed to parse release data: %w", err)
	}
	releaseData = release
	return nil
}

// GetHostPreflights returns a list of HostPreflight specs that are found in the
// binary. These are part of the embedded Kots Application Release.
func GetHostPreflights() (*v1beta2.HostPreflightSpec, error) {
	if err := parseReleaseDataFromBinary(); err != nil {
		return nil, fmt.Errorf("failed to parse data from binary: %w", err)
	}
	return releaseData.GetHostPreflights()
}

// GetLicense reads the kots license from the embedded Kots Application Release. If
// no license is found, returns nil and no error.
func GetLicense() (*v1beta1.License, error) {
	if err := parseReleaseDataFromBinary(); err != nil {
		return nil, fmt.Errorf("failed to parse data from binary: %w", err)
	}
	return releaseData.GetLicense()
}

// GetApplication reads and returns the kots application embedded as part of the
// release. If no application is found, returns nil and no error. This function does
// not unmarshal the application yaml.
func GetApplication() ([]byte, error) {
	if err := parseReleaseDataFromBinary(); err != nil {
		return nil, fmt.Errorf("failed to parse data from binary: %w", err)
	}
	return releaseData.GetApplication()
}

// GetEmbeddedClusterConfig reads the embedded cluster config from the embedded Kots
// Application Release.
func GetEmbeddedClusterConfig() (*embeddedclusterv1beta1.Config, error) {
	if err := parseReleaseDataFromBinary(); err != nil {
		return nil, fmt.Errorf("failed to parse data from binary: %w", err)
	}
	return releaseData.GetEmbeddedClusterConfig()
}

// GetChannelRelease reads the embedded channel release object. If no channel release
// is found, returns nil and no error.
func GetChannelRelease() (*ChannelRelease, error) {
	if err := parseReleaseDataFromBinary(); err != nil {
		return nil, fmt.Errorf("failed to parse data from binary: %w", err)
	}
	return releaseData.GetChannelRelease()
}
