package install

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NamespacesInformer handles ensuring image pull secrets in app namespaces.
// It reads additionalNamespaces from the Application CR, ensures secrets exist
// in those namespaces plus the kotsadm namespace, and watches for new namespace
// creation to deploy secrets to them.
type NamespacesInformer struct {
	kcli             client.Client
	clientset        kubernetes.Interface
	registrySettings *types.RegistrySettings
	logger           logrus.FieldLogger

	watchedNamespaces []string
	stopCh            chan struct{}
	mu                sync.Mutex
}

// runNamespacesInformer creates and starts a syncer that:
// 1. Reads additionalNamespaces from release.GetApplication()
// 2. Immediately ensures image pull secrets and other resources in all watched namespaces
// 3. Starts background informer to watch for new namespace creation
func runNamespacesInformer(
	ctx context.Context,
	kcli client.Client,
	clientset kubernetes.Interface,
	registrySettings *types.RegistrySettings,
	logger logrus.FieldLogger,
) (*NamespacesInformer, error) {
	if registrySettings == nil {
		return nil, fmt.Errorf("registry settings are nil")
	}
	if registrySettings.ImagePullSecretName == "" {
		return nil, fmt.Errorf("image pull secret name is empty")
	}
	if registrySettings.ImagePullSecretValue == "" {
		return nil, fmt.Errorf("image pull secret value is empty")
	}
	if clientset == nil {
		return nil, fmt.Errorf("clientset is nil")
	}

	// Get kotsadm namespace
	kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(ctx, kcli)
	if err != nil {
		return nil, fmt.Errorf("get kotsadm namespace: %w", err)
	}

	// Get watched namespaces from Application CR
	watchedNamespaces := []string{kotsadmNamespace}
	if app := release.GetApplication(); app != nil {
		watchedNamespaces = append(watchedNamespaces, app.Spec.AdditionalNamespaces...)
	}

	s := &NamespacesInformer{
		kcli:              kcli,
		clientset:         clientset,
		registrySettings:  registrySettings,
		logger:            logger,
		watchedNamespaces: watchedNamespaces,
		stopCh:            make(chan struct{}),
	}

	// Immediately ensure secrets in existing namespaces
	for _, ns := range watchedNamespaces {
		if err := s.ensureNamespaceAndResources(ctx, ns); err != nil {
			logger.WithError(err).Warnf("failed to ensure secret in namespace %s", ns)
		}
	}

	// Start background informer
	go s.run()

	return s, nil
}

// Stop stops the background informer
func (s *NamespacesInformer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopCh != nil {
		close(s.stopCh)
		s.stopCh = nil
	}
}

// run watches for namespace creation and ensures secrets
func (s *NamespacesInformer) run() {
	factory := informers.NewSharedInformerFactory(s.clientset, 0)
	nsInformer := factory.Core().V1().Namespaces().Informer()

	_, err := nsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ns, ok := obj.(*corev1.Namespace)
			if !ok {
				return
			}
			if s.shouldWatch(ns.Name) {
				ctx := context.Background()
				if err := s.ensureImagePullSecret(ctx, ns.Name); err != nil {
					s.logger.WithError(err).Warnf("failed to ensure secret in namespace %s", ns.Name)
				}
			}
		},
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to add event handler to namespace informer")
		return
	}

	nsInformer.Run(s.stopCh)
}

// shouldWatch returns true if the namespace is in the watched list
func (s *NamespacesInformer) shouldWatch(namespace string) bool {
	for _, ns := range s.watchedNamespaces {
		if ns == namespace || ns == "*" {
			return true
		}
	}
	return false
}

// ensureNamespaceAndResources creates namespace if needed and ensures required resources exist
func (s *NamespacesInformer) ensureNamespaceAndResources(ctx context.Context, namespace string) error {
	// Create namespace if it doesn't exist
	ns := &corev1.Namespace{}
	err := s.kcli.Get(ctx, client.ObjectKey{Name: namespace}, ns)
	if k8serrors.IsNotFound(err) {
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
		}
		if err := s.kcli.Create(ctx, ns); err != nil && !k8serrors.IsAlreadyExists(err) {
			return fmt.Errorf("create namespace: %w", err)
		}
		s.logger.Infof("created namespace %s", namespace)
	} else if err != nil {
		return fmt.Errorf("get namespace: %w", err)
	}

	if err := s.ensureImagePullSecret(ctx, namespace); err != nil {
		return fmt.Errorf("ensure image pull secret: %w", err)
	}

	return nil
}

// ensureImagePullSecret creates or updates the image pull secret in a namespace
func (s *NamespacesInformer) ensureImagePullSecret(ctx context.Context, namespace string) error {
	secretData, err := base64.StdEncoding.DecodeString(s.registrySettings.ImagePullSecretValue)
	if err != nil {
		return fmt.Errorf("decode secret value: %w", err)
	}

	secret := &corev1.Secret{}
	key := client.ObjectKey{Namespace: namespace, Name: s.registrySettings.ImagePullSecretName}
	err = s.kcli.Get(ctx, key, secret)

	if k8serrors.IsNotFound(err) {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      s.registrySettings.ImagePullSecretName,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{
				".dockerconfigjson": secretData,
			},
		}
		if err := s.kcli.Create(ctx, secret); err != nil {
			return fmt.Errorf("create secret: %w", err)
		}
		s.logger.Infof("created image pull secret %s in namespace %s", s.registrySettings.ImagePullSecretName, namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("get secret: %w", err)
	}

	// Update existing secret
	secret.Data[".dockerconfigjson"] = secretData
	if err := s.kcli.Update(ctx, secret); err != nil {
		return fmt.Errorf("update secret: %w", err)
	}
	s.logger.Infof("updated image pull secret %s in namespace %s", s.registrySettings.ImagePullSecretName, namespace)

	return nil
}
