package embeddedclusteroperator

import (
	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
)

func (e *EmbeddedClusterOperator) prepare(overrides []string) error {
	if err := e.generateHelmValues(overrides); err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	return nil
}

func (e *EmbeddedClusterOperator) generateHelmValues(overrides []string) error {
	for _, override := range overrides {
		var err error
		helmValues, err = helm.PatchValues(helmValues, override)
		if err != nil {
			return errors.Wrap(err, "patch helm values")
		}
	}

	return nil
}
