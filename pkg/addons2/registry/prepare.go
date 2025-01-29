package registry

import (
	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
)

func (r *Registry) prepare(overrides []string) error {
	svcIP, err := helpers.GetLowerBandIP(r.ServiceCIDR, lowerBandIPIndex)
	if err != nil {
		return errors.Wrap(err, "get cluster IP for registry service")
	}
	registryAddress = svcIP.String()

	if err := r.generateHelmValues(overrides); err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	return nil
}

func (r *Registry) generateHelmValues(overrides []string) error {
	var values map[string]interface{}
	if r.IsHA {
		values = helmValuesHA
	} else {
		values = helmValues
	}

	values["tlsSecretName"] = tlsSecretName
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
