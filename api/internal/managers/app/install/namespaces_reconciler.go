package install

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	reconcileInterval = 5 * time.Second
)

// NamespaceReconciler handles ensuring image pull secrets in app namespaces.
// It reads additionalNamespaces from the Application CR, ensures secrets exist
// in those namespaces plus the kotsadm namespace, and polls for new namespace
// creation to deploy secrets to them.
type NamespaceReconciler struct {
	kcli             client.Client
	registrySettings *types.RegistrySettings
	logger           logrus.FieldLogger

	watchedNamespaces []string
	cancel            context.CancelFunc
}

// runNamespaceReconciler creates and starts a reconciler that:
// 1. Reads additionalNamespaces from release.GetApplication()
// 2. Immediately ensures image pull secrets and other resources in all watched namespaces
// 3. Starts background polling to reconcile namespaces periodically
// Returns nil if registry settings are not provided (nothing to reconcile).
func runNamespaceReconciler(
	ctx context.Context,
	kcli client.Client,
	registrySettings *types.RegistrySettings,
	logger logrus.FieldLogger,
) (*NamespaceReconciler, error) {
	// If no registry settings, nothing to do
	if registrySettings == nil || registrySettings.ImagePullSecretName == "" || registrySettings.ImagePullSecretValue == "" {
		return nil, fmt.Errorf("registry settings are nil or empty")
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

	ctx, cancel := context.WithCancel(ctx)

	r := &NamespaceReconciler{
		kcli:              kcli,
		registrySettings:  registrySettings,
		logger:            logger,
		watchedNamespaces: watchedNamespaces,
		cancel:            cancel,
	}

	// Immediately reconcile all namespaces
	r.reconcile(ctx)

	// Start background polling
	go r.run(ctx)

	return r, nil
}

// Stop stops the background reconciler
func (r *NamespaceReconciler) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
}

// run polls periodically to reconcile namespaces
func (r *NamespaceReconciler) run(ctx context.Context) {
	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.reconcile(ctx)
		}
	}
}

// reconcile ensures all watched namespaces have the required resources
func (r *NamespaceReconciler) reconcile(ctx context.Context) {
	namespaces := r.watchedNamespaces

	// If watching all namespaces, list them
	if r.watchesAllNamespaces() {
		nsList := &corev1.NamespaceList{}
		if err := r.kcli.List(ctx, nsList); err != nil {
			r.logger.WithError(err).Warn("failed to list namespaces")
			return
		}
		namespaces = make([]string, 0, len(nsList.Items))
		for _, ns := range nsList.Items {
			namespaces = append(namespaces, ns.Name)
		}
	}

	for _, ns := range namespaces {
		if err := r.reconcileNamespace(ctx, ns); err != nil {
			r.logger.WithError(err).Warnf("failed to reconcile namespace %s", ns)
		}
	}
}

// watchesAllNamespaces returns true if "*" is in the watched namespaces list
func (r *NamespaceReconciler) watchesAllNamespaces() bool {
	for _, ns := range r.watchedNamespaces {
		if ns == "*" {
			return true
		}
	}
	return false
}

// reconcileNamespace creates namespace if needed and ensures required resources exist
func (r *NamespaceReconciler) reconcileNamespace(ctx context.Context, namespace string) error {
	// Skip wildcard entry
	if namespace == "*" {
		return nil
	}

	// Create namespace if it doesn't exist
	ns := &corev1.Namespace{}
	err := r.kcli.Get(ctx, client.ObjectKey{Name: namespace}, ns)
	if k8serrors.IsNotFound(err) {
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
		}
		if err := r.kcli.Create(ctx, ns); err != nil && !k8serrors.IsAlreadyExists(err) {
			return fmt.Errorf("create namespace: %w", err)
		}
		r.logger.Infof("created namespace %s", namespace)
	} else if err != nil {
		return fmt.Errorf("get namespace: %w", err)
	}

	if err := r.ensureImagePullSecret(ctx, namespace); err != nil {
		return fmt.Errorf("ensure image pull secret: %w", err)
	}

	return nil
}

// ensureImagePullSecret creates or updates the image pull secret in a namespace
func (r *NamespaceReconciler) ensureImagePullSecret(ctx context.Context, namespace string) error {
	secretData, err := base64.StdEncoding.DecodeString(r.registrySettings.ImagePullSecretValue)
	if err != nil {
		return fmt.Errorf("decode secret value: %w", err)
	}

	secret := &corev1.Secret{}
	key := client.ObjectKey{Namespace: namespace, Name: r.registrySettings.ImagePullSecretName}
	err = r.kcli.Get(ctx, key, secret)

	if k8serrors.IsNotFound(err) {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      r.registrySettings.ImagePullSecretName,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{
				".dockerconfigjson": secretData,
			},
		}
		if err := r.kcli.Create(ctx, secret); err != nil {
			return fmt.Errorf("create secret: %w", err)
		}
		r.logger.Infof("created image pull secret %s in namespace %s", r.registrySettings.ImagePullSecretName, namespace)
		return nil
	}
	if err != nil {
		return fmt.Errorf("get secret: %w", err)
	}

	// Update existing secret if data differs
	if string(secret.Data[".dockerconfigjson"]) != string(secretData) {
		secret.Data[".dockerconfigjson"] = secretData
		if err := r.kcli.Update(ctx, secret); err != nil {
			return fmt.Errorf("update secret: %w", err)
		}
		r.logger.Infof("updated image pull secret %s in namespace %s", r.registrySettings.ImagePullSecretName, namespace)
	}

	return nil
}
