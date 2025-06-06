package seaweedfs

import (
	"context"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *SeaweedFS) Upgrade(ctx context.Context, logf types.LogFunc, kcli client.Client, mcli metadata.Interface, hcli helm.Client, rc runtimeconfig.RuntimeConfig, overrides []string) error {
	exists, err := hcli.ReleaseExists(ctx, namespace, releaseName)
	if err != nil {
		return errors.Wrap(err, "check if release exists")
	}
	if !exists {
		logrus.Debugf("Release not found, installing release %s in namespace %s", releaseName, namespace)
		return s.Install(ctx, logf, kcli, mcli, hcli, rc, overrides, nil)
	}

	if err := s.ensurePreRequisites(ctx, kcli); err != nil {
		return errors.Wrap(err, "create prerequisites")
	}

	values, err := s.GenerateHelmValues(ctx, kcli, rc, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	_, err = hcli.Upgrade(ctx, helm.UpgradeOptions{
		ReleaseName:  releaseName,
		ChartPath:    s.ChartLocation(),
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
