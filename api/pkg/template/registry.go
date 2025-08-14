package template

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RegistrySettings encapsulates all registry-related configuration
type RegistrySettings struct {
	HasLocalRegistry    bool
	Host                string // e.g., "10.96.0.10:5000"
	Namespace           string // app slug for namespace isolation
	Address             string // full address with namespace prefix
	ImagePullSecretName string // standardized secret name pattern
}

// RegistryDetector defines the interface for detecting registry settings
type RegistryDetector interface {
	DetectRegistrySettings(ctx context.Context, license *kotsv1beta1.License) (*RegistrySettings, error)
}

// KubernetesRegistryDetector detects registry settings from Kubernetes resources
type KubernetesRegistryDetector struct {
	kubeClient client.Client
	logger     logrus.FieldLogger
}

// NewKubernetesRegistryDetector creates a new KubernetesRegistryDetector
func NewKubernetesRegistryDetector(kubeClient client.Client, logger logrus.FieldLogger) *KubernetesRegistryDetector {
	return &KubernetesRegistryDetector{
		kubeClient: kubeClient,
		logger:     logger,
	}
}

// DetectRegistrySettings detects registry settings from Kubernetes cluster
func (k *KubernetesRegistryDetector) DetectRegistrySettings(ctx context.Context, license *kotsv1beta1.License) (*RegistrySettings, error) {
	settings := &RegistrySettings{}

	// Check if registry deployment exists
	deploy := &appsv1.Deployment{}
	err := k.kubeClient.Get(ctx, client.ObjectKey{
		Namespace: constants.RegistryNamespace,
		Name:      "registry",
	}, deploy)

	if err != nil {
		k.logger.WithError(err).Debug("Registry deployment not found")
		return settings, nil // Return empty settings when no registry found
	}

	settings.HasLocalRegistry = true

	// Get registry service for host information
	svc := &corev1.Service{}
	err = k.kubeClient.Get(ctx, client.ObjectKey{
		Namespace: constants.RegistryNamespace,
		Name:      "registry",
	}, svc)

	if err != nil {
		k.logger.WithError(err).Warn("Registry deployment found but service not found")
		return settings, nil
	}

	if svc.Spec.ClusterIP == "" {
		k.logger.Warn("Registry service found but ClusterIP is empty")
		return settings, nil
	}

	settings.Host = fmt.Sprintf("%s:5000", svc.Spec.ClusterIP)

	// Determine namespace from license
	if license != nil && license.Spec.AppSlug != "" {
		settings.Namespace = license.Spec.AppSlug
		settings.ImagePullSecretName = fmt.Sprintf("%s-registry", license.Spec.AppSlug)
	}

	// Set full address
	if settings.Namespace != "" {
		settings.Address = fmt.Sprintf("%s/%s", settings.Host, settings.Namespace)
	} else {
		settings.Address = settings.Host
	}

	return settings, nil
}
