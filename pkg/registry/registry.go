package registry

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster-operator/pkg/util"
)

const (
	// registryNamespace is the namespace where the Registry secret is stored.
	// This namespace is defined in the chart in the release metadata.
	registryNamespace = "registry"

	// registryLowerBandIPIndex is the index of the registry service IP in the service CIDR.
	// this is shared with the CLI as it is set on initial installation as well.
	registryLowerBandIPIndex = 10
)

func RegistryNamespace() string {
	return registryNamespace
}

func GetRegistryServiceIP(serviceCIDR string) (string, error) {
	ip, err := util.GetLowerBandIP(serviceCIDR, registryLowerBandIPIndex)
	if err != nil {
		return "", fmt.Errorf("get lower band ip at index %d: %w", registryLowerBandIPIndex, err)
	}
	return ip.String(), nil
}
