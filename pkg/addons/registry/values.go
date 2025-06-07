package registry

import (
	"context"
	_ "embed"
	"strings"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"gopkg.in/yaml.v3"
)

var (
	//go:embed static/values.tpl.yaml
	rawvalues []byte
	// helmValues is the unmarshal version of rawvalues.
	helmValues map[string]interface{}
	//go:embed static/values-ha.tpl.yaml
	rawvaluesha []byte
	// helmValuesHA is the unmarshal version of rawvaluesha.
	helmValuesHA map[string]interface{}
)

func init() {
	if err := yaml.Unmarshal(rawmetadata, &Metadata); err != nil {
		panic(errors.Wrap(err, "unable to unmarshal metadata"))
	}

	hv, err := release.RenderHelmValues(rawvalues, Metadata)
	if err != nil {
		panic(errors.Wrap(err, "unable to unmarshal values"))
	}
	helmValues = hv

	hvHA, err := release.RenderHelmValues(rawvaluesha, Metadata)
	if err != nil {
		panic(errors.Wrap(err, "unable to unmarshal ha values"))
	}
	helmValuesHA = hvHA
}

func (r *Registry) GenerateHelmValues(ctx context.Context, inSpec ecv1beta1.InstallationSpec, overrides []string) (map[string]interface{}, error) {
	domains := runtimeconfig.GetDomains(inSpec.Config)

	var values map[string]interface{}
	if inSpec.HighAvailability {
		values = helmValuesHA
	} else {
		values = helmValues
	}

	// create a copy of the helm values so we don't modify the original
	marshalled, err := helm.MarshalValues(values)
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

	var serviceCIDR string
	if inSpec.Network != nil && inSpec.Network.ServiceCIDR != "" {
		serviceCIDR = inSpec.Network.ServiceCIDR
	} else {
		var err error
		_, serviceCIDR, err = netutils.SplitNetworkCIDR(ecv1beta1.DefaultNetworkCIDR)
		if err != nil {
			return nil, errors.Wrap(err, "split default network CIDR")
		}
	}

	registryIP, err := GetRegistryClusterIP(serviceCIDR)
	if err != nil {
		return nil, errors.Wrap(err, "get registry cluster IP")
	}
	copiedValues["service"] = map[string]interface{}{
		"clusterIP": registryIP,
	}

	if inSpec.HighAvailability {
		seaweedFSEndpoint, err := seaweedfs.GetS3Endpoint(serviceCIDR)
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
