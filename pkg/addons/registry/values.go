package registry

import (
	"context"
	_ "embed"
	"strings"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	//go:embed static/values.tpl.yaml
	rawvalues []byte
	//go:embed static/values-ha.tpl.yaml
	rawvaluesha []byte
)

func (r *Registry) GenerateHelmValues(ctx context.Context, kcli client.Client, domains ecv1beta1.Domains, overrides []string) (map[string]interface{}, error) {
	var hv map[string]interface{}
	if r.IsHA {
		v, err := helmValuesHA()
		if err != nil {
			return nil, errors.Wrap(err, "get helm values ha")
		}
		hv = v
	} else {
		v, err := helmValues()
		if err != nil {
			return nil, errors.Wrap(err, "get helm values")
		}
		hv = v
	}

	marshalled, err := helm.MarshalValues(hv)
	if err != nil {
		return nil, errors.Wrap(err, "marshal helm values")
	}

	// replace proxy.replicated.com with the potentially customized proxy registry domain
	if domains.ProxyRegistryDomain != "" {
		marshalled = strings.ReplaceAll(marshalled, "proxy.replicated.com", domains.ProxyRegistryDomain)
	}

	copiedValues, err := helm.UnmarshalValues(marshalled)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshal helm values")
	}

	// only add tls secret value if the secret exists
	// this is for backwards compatibility when the registry was deployed without TLS
	var secret corev1.Secret
	if err := kcli.Get(ctx, client.ObjectKey{Namespace: r.Namespace(), Name: _tlsSecretName}, &secret); err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, errors.Wrap(err, "get tls secret")
		}
	} else {
		copiedValues["tlsSecretName"] = _tlsSecretName
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

func helmValues() (map[string]interface{}, error) {
	if err := yaml.Unmarshal(rawmetadata, &Metadata); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata")
	}

	hv, err := release.RenderHelmValues(rawvalues, Metadata)
	if err != nil {
		return nil, errors.Wrap(err, "render helm values")
	}

	return hv, nil
}

func helmValuesHA() (map[string]interface{}, error) {
	hvHA, err := release.RenderHelmValues(rawvaluesha, Metadata)
	if err != nil {
		return nil, errors.Wrap(err, "render helm values")
	}

	return hvHA, nil
}
