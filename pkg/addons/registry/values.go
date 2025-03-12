package registry

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Registry) GenerateHelmValues(ctx context.Context, kcli client.Client, overrides []string) (map[string]interface{}, error) {
	var values map[string]interface{}
	if r.IsHA {
		values = helmValuesHA
	} else {
		values = helmValues
	}

	// create a copy of the helm values so we don't modify the original
	marshalled, err := helm.MarshalValues(values)
	if err != nil {
		return nil, errors.Wrap(err, "marshal helm values")
	}

	proxyRegistryDomain := runtimeconfig.ProxyRegistryDomain()
	// replace proxy.replicated.com with the potentially customized proxy registry domain
	marshalled = strings.ReplaceAll(marshalled, "proxy.replicated.com", proxyRegistryDomain)

	copiedValues, err := helm.UnmarshalValues(marshalled)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshal helm values")
	}

	// only add tls secret value if the secret exists
	// this is for backwards compatibility when the registry was deployed without TLS
	var secret corev1.Secret
	if err := kcli.Get(ctx, k8stypes.NamespacedName{Namespace: namespace, Name: tlsSecretName}, &secret); err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, errors.Wrap(err, "get tls secret")
		}
	} else {
		copiedValues["tlsSecretName"] = tlsSecretName
	}

	registryIP, err := GetRegistryClusterIP(r.ServiceCIDR)
	if err != nil {
		return nil, errors.Wrap(err, "get registry cluster IP")
	}
	copiedValues["service"] = map[string]interface{}{
		"clusterIP": registryIP,
	}

	if r.IsHA {
		seaweedFSEndpoint, err := seaweedfs.GetS3Endpoint(r.ServiceCIDR)
		if err != nil {
			return nil, errors.Wrap(err, "get seaweedfs s3 endpoint")
		}
		copiedValues["s3"].(map[string]interface{})["regionEndpoint"] = seaweedFSEndpoint
	}

	for _, override := range overrides {
		var err error
		copiedValues, err = helm.PatchValues(copiedValues, override)
		if err != nil {
			return nil, errors.Wrap(err, "patch helm values")
		}
	}

	return copiedValues, nil
}
