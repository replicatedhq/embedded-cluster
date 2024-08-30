package registry

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/operator/pkg/util"
)

const (
	// seaweedfsLowerBandIPIndex is the index of the seaweedfs service IP in the service CIDR.
	// HACK: this is shared with the cli and operator as it is used by the registry to redirect requests for blobs.
	seaweedfsLowerBandIPIndex = 11
)

func GetSeaweedfsS3Endpoint(serviceCIDR string) (string, error) {
	ip, err := getSeaweedfsS3ServiceIP(serviceCIDR)
	if err != nil {
		return "", fmt.Errorf("get seaweedfs s3 service IP: %w", err)
	}
	return fmt.Sprintf("%s:8333", ip), nil
}

func getSeaweedfsS3ServiceIP(serviceCIDR string) (string, error) {
	ip, err := util.GetLowerBandIP(serviceCIDR, seaweedfsLowerBandIPIndex)
	if err != nil {
		return "", fmt.Errorf("get lower band ip at index %d: %w", seaweedfsLowerBandIPIndex, err)
	}
	return ip.String(), nil
}
