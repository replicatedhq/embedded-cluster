package restoreplan

import (
	_ "embed"
	"fmt"
	"strings"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

const (
	airgapAnnotation           = "kots.io/is-airgap"
	highAvailabilityAnnotation = "kots.io/embedded-cluster-is-ha"
	registryAnnotation         = "kots.io/embedded-registry"
	seaweedFSAnnotation        = "kots.io/embedded-cluster-seaweedfs-s3-ip"
)

//go:embed resource-modifiers.yaml
var resourceModifiersYAML string

// ResourceModifiers renders the Velero resource-modifier configuration used
// by the restore command. Keeping this outside the CLI lets tests exercise the
// exact production restore plan.
func ResourceModifiers(backup *velerov1.Backup) (string, error) {
	registryIP, err := RegistryServiceIP(backup)
	if err != nil {
		return "", fmt.Errorf("get registry service IP: %w", err)
	}
	seaweedFSIP, err := SeaweedFSS3ServiceIP(backup)
	if err != nil {
		return "", fmt.Errorf("get SeaweedFS S3 service IP: %w", err)
	}
	rendered := strings.Replace(resourceModifiersYAML, "__REGISTRY_SERVICE_IP__", registryIP, 1)
	rendered = strings.Replace(rendered, "__SEAWEEDFS_S3_SERVICE_IP__", seaweedFSIP, 1)
	return rendered, nil
}

// RegistryServiceIP returns no IP for an online backup and requires the
// preserved registry endpoint for an airgap backup.
func RegistryServiceIP(backup *velerov1.Backup) (string, error) {
	isAirgap, ok := backup.Annotations[airgapAnnotation]
	if !ok {
		return "", fmt.Errorf("airgap annotation not found")
	}
	if isAirgap != "true" {
		return "", nil
	}
	host, ok := backup.Annotations[registryAnnotation]
	if !ok {
		return "", fmt.Errorf("embedded registry service IP annotation not found")
	}
	return strings.Split(host, ":")[0], nil
}

// SeaweedFSS3ServiceIP returns no IP unless a backup is both airgapped and HA.
func SeaweedFSS3ServiceIP(backup *velerov1.Backup) (string, error) {
	isAirgap, ok := backup.Annotations[airgapAnnotation]
	if !ok {
		return "", fmt.Errorf("airgap annotation not found")
	}
	if isAirgap != "true" {
		return "", nil
	}
	isHA, ok := backup.Annotations[highAvailabilityAnnotation]
	if !ok {
		return "", fmt.Errorf("high availability annotation not found")
	}
	if isHA != "true" {
		return "", nil
	}
	ip, ok := backup.Annotations[seaweedFSAnnotation]
	if !ok {
		return "", fmt.Errorf("SeaweedFS S3 service IP annotation not found")
	}
	return ip, nil
}
