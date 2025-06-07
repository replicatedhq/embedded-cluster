package seaweedfs

import (
	"context"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
)

func (s *SeaweedFS) Upgrade(ctx context.Context, clients types.Clients, inSpec ecv1beta1.InstallationSpec, overrides []string) error {
	exists, err := clients.HelmClient.ReleaseExists(ctx, s.Namespace(), releaseName)
	if err != nil {
		return errors.Wrap(err, "check if release exists")
	}
	if !exists {
		logrus.Debugf("Release not found, installing release %s in namespace %s", releaseName, s.Namespace())
		return s.Install(ctx, clients, nil, inSpec, overrides, types.InstallOptions{})
	}

	if err := s.ensurePreRequisites(ctx, clients, inSpec); err != nil {
		return errors.Wrap(err, "create prerequisites")
	}

	values, err := s.GenerateHelmValues(ctx, inSpec, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	_, err = clients.HelmClient.Upgrade(ctx, helm.UpgradeOptions{
		ReleaseName:  releaseName,
		ChartPath:    s.ChartLocation(runtimeconfig.GetDomains(inSpec.Config)),
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    s.Namespace(),
		Labels:       getBackupLabels(),
		Force:        false,
	})
	if err != nil {
		return errors.Wrap(err, "helm upgrade")
	}

	return nil
}
