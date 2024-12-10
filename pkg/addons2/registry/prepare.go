package registry

import (
	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
)

func (r *Registry) Prepare() error {
	svcIP, err := helpers.GetLowerBandIP(r.ServiceCIDR, registryLowerBandIPIndex)
	if err != nil {
		return errors.Wrap(err, "get cluster IP for registry service")
	}
	registryAddress = svcIP.String()

	if err := r.generateHelmValues(); err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	return nil
}

func (r *Registry) generateHelmValues() error {
	helmValues["tlsSecretName"] = tlsSecretName

	helmValues["service"] = map[string]interface{}{
		"clusterIP": registryAddress,
	}

	return nil
}
