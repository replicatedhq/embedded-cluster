package embeddedclusteroperator

import (
	"github.com/pkg/errors"
)

func (a *EmbeddedClusterOperator) prepare() error {
	if err := a.generateHelmValues(); err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	return nil
}

func (a *EmbeddedClusterOperator) generateHelmValues() error {
	return nil
}
