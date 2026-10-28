package restoreplan

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	airgapAnnotation           = "kots.io/is-airgap"
	highAvailabilityAnnotation = "kots.io/embedded-cluster-is-ha"
	registryAnnotation         = "kots.io/embedded-registry"
	seaweedFSAnnotation        = "kots.io/embedded-cluster-seaweedfs-s3-ip"
)

const ResourceModifiersConfigMapName = "restore-resource-modifiers"

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

// EnsureResourceModifiers creates the ConfigMap referenced by production
// Velero Restore objects. Existing restore plans are retained so a resumed
// restore uses the same immutable inputs as its original attempt.
func EnsureResourceModifiers(ctx context.Context, kcli client.Client, backup *velerov1.Backup) error {
	rendered, err := ResourceModifiers(backup)
	if err != nil {
		return err
	}
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: constants.VeleroNamespace,
			Name:      ResourceModifiersConfigMapName,
		},
		Data: map[string]string{"resource-modifiers.yaml": rendered},
	}
	if err := kcli.Create(ctx, configMap); err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("create resource-modifier ConfigMap: %w", err)
	}
	return nil
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
