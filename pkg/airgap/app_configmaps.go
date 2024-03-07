package airgap

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateAppConfigMaps(cli client.Client) error {

	// TODO: for each file in the app release, create a configmap
	return nil
}

func createAppConfigMap(ctx context.Context, cli client.Client, key string, contents []byte) error {
	rel, err := release.GetChannelRelease()
	if err != nil {
		return fmt.Errorf("failed to get channel release: %w", err)
	}

	configMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("kotsadm-airgap-%s", key),
			Namespace: defaults.KOTSADM_NAMESPACE,
			Labels: map[string]string{
				"kots.io/automation": "airgap",
				"kots.io/app":        rel.AppSlug,
				"kots.io/kotsadm":    "true",
			},
		},
		Data: map[string]string{
			key: base64.StdEncoding.EncodeToString(contents),
		},
	}

	err = cli.Create(ctx, configMap)
	if err != nil {
		return fmt.Errorf("failed to create configmap %s: %w", configMap.Name, err)
	}

	return nil
}
