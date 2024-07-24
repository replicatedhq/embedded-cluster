package highavailability

import (
	"context"
	"fmt"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// registryS3SecretName is the name of the Registry secret.
	// This secret name is defined in the chart in the release metadata.
	registryS3SecretName = "seaweedfs-s3-rw"
)

func createRegistryResources(ctx context.Context, kcli client.Client, config *seaweedfsConfig) error {
	err := ensureRegistryNamespace(ctx, kcli)
	if err != nil {
		return fmt.Errorf("unable to create registry namespace: %w", err)
	}

	err = kubeutils.WaitForNamespace(ctx, kcli, defaults.RegistryNamespace)
	if err != nil {
		return fmt.Errorf("wait for registry namespace: %w", err)
	}

	err = ensureRegistryS3Secret(ctx, kcli, config)
	if err != nil {
		return fmt.Errorf("unable to create registry s3 secret: %w", err)
	}

	return nil
}

func ensureRegistryNamespace(ctx context.Context, cli client.Client) error {
	obj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: defaults.RegistryNamespace},
	}

	err := cli.Create(ctx, obj)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("create registry namespace: %w", err)

	}

	return nil
}

func ensureRegistryS3Secret(ctx context.Context, cli client.Client, config *seaweedfsConfig) error {
	sfsCreds, ok := config.getCredentials("anvAdmin")
	if !ok {
		return fmt.Errorf("seaweedfs s3 anvAdmin credentials not found")
	}

	obj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: registryS3SecretName, Namespace: defaults.RegistryNamespace},
		Data: map[string][]byte{
			"s3AccessKey": []byte(sfsCreds.AccessKey),
			"s3SecretKey": []byte(sfsCreds.SecretKey),
		},
	}

	obj.ObjectMeta.Labels = applySeaweedFSLabels(obj.ObjectMeta.Labels, "s3")

	err := cli.Create(ctx, obj)
	if err != nil {
		return fmt.Errorf("create registry seaweedfs s3 service: %w", err)
	}
	return nil
}
