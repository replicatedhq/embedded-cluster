package seaweedfs

import (
	"context"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *SeaweedFS) Upgrade(
	ctx context.Context, logf types.LogFunc,
	kcli client.Client, mcli metadata.Interface, hcli helm.Client,
	domains ecv1beta1.Domains, overrides []string,
) error {
	logrus.Debugf("SeaweedFS.Upgrade: starting upgrade for release '%s' in namespace '%s'", s.ReleaseName(), s.Namespace())

	exists, err := hcli.ReleaseExists(ctx, s.Namespace(), s.ReleaseName())
	if err != nil {
		logrus.Debugf("SeaweedFS.Upgrade: ReleaseExists failed: %v", err)
		return errors.Wrap(err, "check if release exists")
	}

	logrus.Debugf("SeaweedFS.Upgrade: ReleaseExists returned %t", exists)

	if !exists {
		logrus.Debugf("SeaweedFS.Upgrade: Release not found, installing release %s in namespace %s", s.ReleaseName(), s.Namespace())
		return s.Install(ctx, logf, kcli, mcli, hcli, domains, overrides)
	}

	logrus.Debugf("SeaweedFS.Upgrade: Release exists, proceeding with helm upgrade")

	if err := s.ensurePreRequisites(ctx, kcli); err != nil {
		return errors.Wrap(err, "create prerequisites")
	}

	values, err := s.GenerateHelmValues(ctx, kcli, domains, overrides)
	if err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	_, err = hcli.Upgrade(ctx, helm.UpgradeOptions{
		ReleaseName:  s.ReleaseName(),
		ChartPath:    s.ChartLocation(domains),
		ChartVersion: Metadata.Version,
		Values:       values,
		Namespace:    s.Namespace(),
		Labels:       getBackupLabels(),
		Force:        false,
		LogFn:        helm.LogFn(logf),
	})
	if err != nil {
		return errors.Wrap(err, "helm upgrade")
	}

	return nil
}
