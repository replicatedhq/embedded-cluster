package seaweedfs

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *SeaweedFS) Upgrade(ctx context.Context, kcli client.Client, hcli helm.Client, overrides []string) error {
	if err := s.ensurePreRequisites(ctx, kcli); err != nil {
		return errors.Wrap(err, "create prerequisites")
	}

	values, err := s.GenerateHelmValues(ctx, kcli, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	exists, err := hcli.ReleaseExists(ctx, namespace, releaseName)
	if err != nil {
		return errors.Wrap(err, "check if release exists")
	}
	if !exists {
		logrus.Debugf("Release not found, installing release %s in namespace %s", releaseName, namespace)
		_, err = hcli.Install(ctx, helm.InstallOptions{
			ReleaseName:  releaseName,
			ChartPath:    Metadata.Location,
			ChartVersion: Metadata.Version,
			Values:       values,
			Namespace:    namespace,
			Labels:       getBackupLabels(),
		})
		if err != nil {
			return errors.Wrap(err, "helm install")
		}
		return nil
	}

	_, err = hcli.Upgrade(ctx, helm.UpgradeOptions{
		ReleaseName:  releaseName,
		ChartPath:    Metadata.Location,
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    namespace,
		Labels:       getBackupLabels(),
		Force:        false,
	})
	if err != nil {
		return errors.Wrap(err, "helm upgrade")
	}

	return nil
}
