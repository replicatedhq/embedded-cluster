package seaweedfs

import (
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

func (s *SeaweedFS) prepare(overrides []string) error {
	if err := s.generateHelmValues(overrides); err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	return nil
}

func (s *SeaweedFS) generateHelmValues(overrides []string) error {
	var err error
	dataPath := filepath.Join(runtimeconfig.EmbeddedClusterSeaweedfsSubDir(), "ssd")
	helmValues, err = helm.SetValue(helmValues, "master.data.hostPathPrefix", dataPath)
	if err != nil {
		return errors.Wrap(err, "set helm values global.data.hostPathPrefix")
	}

	logsPath := filepath.Join(runtimeconfig.EmbeddedClusterSeaweedfsSubDir(), "storage")
	helmValues, err = helm.SetValue(helmValues, "master.logs.hostPathPrefix", logsPath)
	if err != nil {
		return errors.Wrap(err, "set helm values global.logs.hostPathPrefix")
	}

	for _, override := range overrides {
		helmValues, err = helm.PatchValues(helmValues, override)
		if err != nil {
			return errors.Wrap(err, "patch helm values")
		}
	}

	return nil
}
