package embeddedclusteroperator

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const supportSpec = `
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: infrastructure-logs
spec:
  collectors:
    - logs:
        name: embedded-cluster
        namespace: embedded-cluster
        limits:
          maxAge: 7200h
          maxLines: 10000
    - logs:
        name: openebs
        namespace: openebs
        limits:
          maxAge: 7200h
          maxLines: 10000
    - logs:
        name: registry
        namespace: registry
        limits:
          maxAge: 7200h
          maxLines: 10000
`

func createSupportSpec(ctx context.Context, cli client.Client) error {
	supportConfig := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "embedded-cluster-operator-support",
			Namespace: "embedded-cluster",
			Labels: map[string]string{
				"troubleshoot.sh/kind": "support-bundle",
			},
		},
		Data: map[string]string{
			"support-bundle-spec": supportSpec,
		},
	}

	if err := cli.Create(ctx, &supportConfig); err != nil {
		return err
	}

	return nil
}
