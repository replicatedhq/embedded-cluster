package install

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// namespaceReconciler handles ensuring image pull secrets and CA configmaps in app namespaces.
// It reads additionalNamespaces from the Application CR and ensures secrets and configmaps exist
// in those namespaces plus the kotsadm namespace.
type namespaceReconciler struct {
	kcli             client.Client
	mcli             metadata.Interface
	registrySettings *types.RegistrySettings
	hostCABundlePath string
	appSlug          string
	versionLabel     string
	logger           logrus.FieldLogger

	namespaces []string
}

// newNamespaceReconciler creates a new namespace reconciler
func newNamespaceReconciler(
	ctx context.Context,
	kcli client.Client,
	mcli metadata.Interface,
	registrySettings *types.RegistrySettings,
	hostCABundlePath string,
	appSlug string,
	versionLabel string,
	logger logrus.FieldLogger,
) (*namespaceReconciler, error) {
	// Get kotsadm namespace
	kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(ctx, kcli)
	if err != nil {
		return nil, fmt.Errorf("get kotsadm namespace: %w", err)
	}

	// Get watched namespaces from Application CR
	watchedNamespaces := []string{kotsadmNamespace}
	if app := release.GetApplication(); app != nil {
		for _, ns := range app.Spec.AdditionalNamespaces {
			// NOTE: we no longer support watching all namespaces ("*")
			if ns == "*" {
				logger.Warn("watching all namespaces is not supported (\"*\")")
			} else {
				watchedNamespaces = append(watchedNamespaces, ns)
			}
		}
	}

	r := &namespaceReconciler{
		kcli:             kcli,
		mcli:             mcli,
		registrySettings: registrySettings,
		hostCABundlePath: hostCABundlePath,
		appSlug:          appSlug,
		versionLabel:     versionLabel,
		logger:           logger,
		namespaces:       watchedNamespaces,
	}

	return r, nil
}

// reconcile ensures all watched namespaces have the required resources
func (r *namespaceReconciler) reconcile(ctx context.Context) error {
	for _, ns := range r.namespaces {
		if err := r.reconcileNamespace(ctx, ns); err != nil {
			return fmt.Errorf("reconcile namespace %s: %w", ns, err)
		}
	}
	return nil
}

// reconcileNamespace creates namespace if needed and ensures required resources exist
func (r *namespaceReconciler) reconcileNamespace(ctx context.Context, namespace string) error {
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

	if err := r.ensureCAConfigmap(ctx, namespace); err != nil {
		return fmt.Errorf("ensure ca configmap: %w", err)
	}

	return nil
}

// ensureImagePullSecret creates or updates the image pull secret in a namespace
func (r *namespaceReconciler) ensureImagePullSecret(ctx context.Context, namespace string) error {
	// Skip if no registry settings
	if r.registrySettings == nil || r.registrySettings.ImagePullSecretName == "" || r.registrySettings.ImagePullSecretValue == "" {
		return nil
	}

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
				Labels:    utils.GetK8sObjectMetaLabels(r.appSlug, r.versionLabel, "registry"),
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

// ensureCAConfigmap ensures the CA configmap exists in the namespace
func (r *namespaceReconciler) ensureCAConfigmap(ctx context.Context, namespace string) error {
	// Skip if no CA bundle path
	if r.hostCABundlePath == "" {
		return nil
	}

	return adminconsole.EnsureCAConfigmap(ctx, r.logger.Infof, r.kcli, r.mcli, namespace, r.hostCABundlePath)
}
