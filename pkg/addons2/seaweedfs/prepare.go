package seaweedfs

import (
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

func (s *SeaweedFS) prepare() error {
	if err := s.generateHelmValues(); err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	return nil
}

func (s *SeaweedFS) generateHelmValues() error {
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

	return nil
}
