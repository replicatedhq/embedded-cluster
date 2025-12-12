package config

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// readConfigValuesFromKube reads existing config values from Kubernetes
func (m *appConfigManager) readConfigValuesFromKube() (types.AppConfigValues, error) {
	if m.kcli == nil {
		m.logger.Warn("kube client not initialized, proceeding without reading config values from kubernetes")
		return types.AppConfigValues{}, nil
	}

	kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(context.TODO(), m.kcli)
	if err != nil {
		return nil, fmt.Errorf("get kotsadm namespace: %w", err)
	}

	// Try to get existing config values secret, if not found return early
	secret := &corev1.Secret{}
	if err := m.kcli.Get(context.TODO(), client.ObjectKey{
		Name:      utils.GetConfigValuesSecretName(m.releaseData.ChannelRelease.AppSlug),
		Namespace: kotsadmNamespace,
	}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return types.AppConfigValues{}, nil
		}
		return nil, fmt.Errorf("get existing config values secret: %w", err)
	}

	configValues, err := utils.ReadConfigValuesFromSecret(secret)
	if err != nil {
		return nil, fmt.Errorf("read config values from secret: %w", err)
	}
	appConfigValues := types.ConvertToAppConfigValues(configValues)

	return appConfigValues, nil
}
