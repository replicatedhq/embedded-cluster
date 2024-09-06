package artifacts

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	clusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	RegistryCredsSecretName = "registry-creds"

	kotsadmNamespace = "kotsadm"
)

// dockerConfig represents the content of the '.dockerconfigjson' secret.
type dockerConfig struct {
	Auths map[string]dockerConfigEntry `json:"auths"`
}

// dockerConfigEntry represents the content of the '.dockerconfigjson' secret.
type dockerConfigEntry struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func EnsureRegistrySecretInECNamespace(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation) (controllerutil.OperationResult, error) {
	op := controllerutil.OperationResultNone

	nsn := types.NamespacedName{Name: RegistryCredsSecretName, Namespace: kotsadmNamespace}
	var kotsadmSecret corev1.Secret
	err := cli.Get(ctx, nsn, &kotsadmSecret)
	if err != nil {
		return op, fmt.Errorf("get secret in kotsadm namespace: %w", err)
	}

	obj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: RegistryCredsSecretName, Namespace: ecNamespace},
	}

	op, err = ctrl.CreateOrUpdate(ctx, cli, obj, func() error {
		if in.GetUID() != "" {
			err := ctrl.SetControllerReference(in, obj, cli.Scheme())
			if err != nil {
				return fmt.Errorf("set controller reference: %w", err)
			}
		}

		obj.ObjectMeta.Labels = applyECOperatorLabels(obj.ObjectMeta.Labels, "upgrader")

		obj.Type = corev1.SecretTypeDockerConfigJson
		obj.Data = kotsadmSecret.Data

		return nil
	})
	if err != nil {
		return op, fmt.Errorf("create or update registry creds secret: %w", err)
	}

	return op, nil
}

func GetRegistryImagePullSecret() corev1.LocalObjectReference {
	return corev1.LocalObjectReference{Name: RegistryCredsSecretName}
}

// registryAuth returns the authentication store to be used when reaching the
// registry. The authentication store is read from the cluster secret named
// 'registry-creds' in the 'kotsadm' namespace.
func registryAuth(ctx context.Context, log logr.Logger, cli client.Client) (credentials.Store, error) {
	nsn := types.NamespacedName{Name: RegistryCredsSecretName, Namespace: kotsadmNamespace}
	var sct corev1.Secret
	if err := cli.Get(ctx, nsn, &sct); err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, fmt.Errorf("get secret: %w", err)
		}
		log.Info("Secret registry-creds not found, using anonymous access")
		return credentials.NewMemoryStore(), nil
	}

	data, ok := sct.Data[".dockerconfigjson"]
	if !ok {
		return nil, fmt.Errorf("secret does not contain .dockerconfigjson")
	}

	var cfg dockerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal secret: %w", err)
	}

	creds := credentials.NewMemoryStore()
	for addr, entry := range cfg.Auths {
		err := creds.Put(ctx, addr, auth.Credential{
			Username: entry.Username,
			Password: entry.Password,
		})
		if err != nil {
			return nil, fmt.Errorf("put credential for %s: %w", addr, err)
		}
	}
	return creds, nil
}

func applyECOperatorLabels(labels map[string]string, component string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["app.kubernetes.io/component"] = component
	labels["app.kubernetes.io/part-of"] = "embedded-cluster"
	labels["app.kubernetes.io/managed-by"] = "embedded-cluster-operator"
	return labels
}
