package utils

import (
	"fmt"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kyaml "sigs.k8s.io/yaml"
)

// TODO consider moving the logic in this file to the app config manager or something similar where we consolodiate access to app config

// GetConfigValuesSecretName returns the name of the secret used to store config values for the given app slug
func GetConfigValuesSecretName(appSlug string) string {
	return fmt.Sprintf("%s-config-values", appSlug)
}

// GenerateConfigValueSecret generates a Kubernetes Secret object to store config values
func GenerateConfigValueSecret(configValuesData []byte, appSlug string, namespace string, versionLabel string) *corev1.Secret {
	// Create secret object
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetConfigValuesSecretName(appSlug),
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       appSlug,
				"app.kubernetes.io/version":    versionLabel,
				"app.kubernetes.io/component":  "config",
				"app.kubernetes.io/part-of":    "embedded-cluster",
				"app.kubernetes.io/managed-by": "embedded-cluster-installer",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"config-values.yaml": configValuesData,
		},
	}
}

// ReadConfigValuesFromSecret reads config values from the given Kubernetes Secret
func ReadConfigValuesFromSecret(secret *corev1.Secret) (*kotsv1beta1.ConfigValues, error) {
	data, exists := secret.Data["config-values.yaml"]
	if !exists {
		return nil, fmt.Errorf("config-values.yaml key not found in secret %s", secret.Name)
	}
	var configValues kotsv1beta1.ConfigValues
	if err := kyaml.Unmarshal(data, &configValues); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config values data: %w", err)
	}
	return &configValues, nil
}
