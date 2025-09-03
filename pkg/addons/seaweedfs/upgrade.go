package seaweedfs

import (
	"context"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *SeaweedFS) Upgrade(
	ctx context.Context, logf types.LogFunc,
	kcli client.Client, mcli metadata.Interface, hcli helm.Client,
	domains ecv1beta1.Domains, overrides []string,
) error {
	exists, err := hcli.ReleaseExists(ctx, s.Namespace(), s.ReleaseName())
	if err != nil {
		return errors.Wrap(err, "check if release exists")
	}
	if !exists {
		logrus.Debugf("Release not found, installing release %s in namespace %s", s.ReleaseName(), s.Namespace())
		return s.Install(ctx, logf, kcli, mcli, hcli, domains, overrides)
	}

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
	})
	if err != nil {
		return errors.Wrap(err, "helm upgrade")
	}

	// Check if this upgrade needs SeaweedFS master pod restart for scaling fix
	if needsScalingRestart(ctx, kcli) {
		logf("Restarting SeaweedFS master pods after scaling to ensure proper quorum setup")

		if err := restartMasterPods(ctx, kcli); err != nil {
			return errors.Wrap(err, "failed to restart SeaweedFS master pods after scaling")
		}
	}

	return nil
}

// needsScalingRestart checks if this upgrade requires SeaweedFS master pod restart
// due to scaling from single replica to HA mode from versions < 2.8.1
func needsScalingRestart(ctx context.Context, kcli client.Client) bool {
	// Get the latest installation to use for getting the previous one
	latest, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		logrus.Debugf("Could not get latest installation: %v", err)
		return false
	}

	// Get the previous installation to check version
	previous, err := kubeutils.GetPreviousInstallation(ctx, kcli, latest)
	if err != nil {
		logrus.Debugf("Could not get previous installation: %v", err)
		return false
	}

	if previous == nil || previous.Spec.Config == nil || previous.Spec.Config.Version == "" {
		logrus.Debug("No previous installation or version found, skipping restart")
		return false
	}

	// Parse previous version
	prevVersion, err := semver.NewVersion(previous.Spec.Config.Version)
	if err != nil {
		logrus.Debugf("Could not parse previous version %s: %v", previous.Spec.Config.Version, err)
		return false
	}

	// Only restart if upgrading from < 2.8.1
	if !lessThanECVersion281(prevVersion) {
		logrus.Debug("Previous version >= 2.8.1, no restart needed")
		return false
	}

	// Check if SeaweedFS StatefulSet exists and shows scaling in progress
	var sts appsv1.StatefulSet
	nsn := client.ObjectKey{Namespace: "seaweedfs", Name: "seaweedfs-master"}
	if err := kcli.Get(ctx, nsn, &sts); err != nil {
		logrus.Debugf("Could not get SeaweedFS master StatefulSet: %v", err)
		return false
	}

	// If replicas is 3 but not all are ready, we likely need to restart
	if sts.Spec.Replicas != nil && *sts.Spec.Replicas == 3 && sts.Status.ReadyReplicas < 3 {
		logrus.Debug("SeaweedFS scaling detected (3 replicas configured, not all ready)")
		return true
	}

	return false
}

// lessThanECVersion281 checks if a version is less than 2.8.1
func lessThanECVersion281(ver *semver.Version) bool {
	version281 := semver.MustParse("2.8.1")
	return ver.LessThan(version281)
}

// restartMasterPods restarts the SeaweedFS master pods in sequence to ensure
// they pick up the correct peer configuration after scaling
func restartMasterPods(ctx context.Context, kcli client.Client) error {
	kubeutils := &kubeutils.KubeUtils{}

	// Restart SeaweedFS master pods in sequence
	if err := kubeutils.RestartStatefulSetPods(ctx, kcli, "seaweedfs", "seaweedfs-master"); err != nil {
		return errors.Wrap(err, "restart SeaweedFS master pods")
	}

	// Wait for the StatefulSet to be fully ready after restart
	if err := kubeutils.WaitForStatefulset(ctx, kcli, "seaweedfs", "seaweedfs-master", nil); err != nil {
		return errors.Wrap(err, "wait for SeaweedFS master StatefulSet to be ready")
	}

	logrus.Info("SeaweedFS master pods restarted successfully")
	return nil
}
