package openebs

import (
	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

func (o *OpenEBS) prepare(overrides []string) error {
	if err := o.generateHelmValues(overrides); err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	return nil
}

func (o *OpenEBS) generateHelmValues(overrides []string) error {
	var err error
	helmValues, err = helm.SetValue(helmValues, `["localpv-provisioner"].localpv.basePath`, runtimeconfig.EmbeddedClusterOpenEBSLocalSubDir())
	if err != nil {
		return errors.Wrap(err, "set localpv-provisioner.localpv.basePath")
	}

	for _, override := range overrides {
		helmValues, err = helm.PatchValues(helmValues, override)
		if err != nil {
			return errors.Wrap(err, "patch helm values")
		}
	}

	return nil
}
