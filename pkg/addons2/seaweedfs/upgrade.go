package seaweedfs

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *SeaweedFS) Upgrade(ctx context.Context, kcli client.Client, hcli *helm.Helm) error {
	if err := s.prepare(); err != nil {
		return errors.Wrap(err, "prepare seaweedfs")
	}

	_, err := hcli.Upgrade(ctx, helm.UpgradeOptions{
		ReleaseName:  releaseName,
		ChartPath:    Metadata.Location,
		ChartVersion: Metadata.Version,
		Values:       helmValues,
		Namespace:    namespace,
		Force:        false,
	})
	if err != nil {
		return errors.Wrap(err, "upgrade seaweedfs")
	}

	return nil
}
