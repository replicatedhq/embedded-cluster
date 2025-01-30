package registry

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Registry) prepare(ctx context.Context, kcli client.Client, overrides []string) error {
	svcIP, err := helpers.GetLowerBandIP(r.ServiceCIDR, lowerBandIPIndex)
	if err != nil {
		return errors.Wrap(err, "get cluster IP for registry service")
	}
	registryAddress = svcIP.String()

	if err := r.generateHelmValues(ctx, kcli, overrides); err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	return nil
}

func (r *Registry) generateHelmValues(ctx context.Context, kcli client.Client, overrides []string) error {
	var values map[string]interface{}
	if r.IsHA {
		values = helmValuesHA
	} else {
		values = helmValues
	}

	// only add tls secret value if the secret exists
	// this is for backwards compatibility when the registry was deployed without TLS
	var secret corev1.Secret
	if err := kcli.Get(ctx, k8stypes.NamespacedName{Namespace: namespace, Name: tlsSecretName}, &secret); err == nil {
		values["tlsSecretName"] = tlsSecretName
	}

	values["service"] = map[string]interface{}{
		"clusterIP": registryAddress,
	}

	if r.IsHA {
		seaweedFSEndpoint, err := seaweedfs.GetS3Endpoint(r.ServiceCIDR)
		if err != nil {
			return errors.Wrap(err, "get seaweedfs s3 endpoint")
		}
		values["s3"].(map[string]interface{})["regionEndpoint"] = seaweedFSEndpoint
	}

	for _, override := range overrides {
		var err error
		helmValues, err = helm.PatchValues(helmValues, override)
		if err != nil {
			return errors.Wrap(err, "patch helm values")
		}
	}

	return nil
}
