package seaweedfs

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	if needsScalingRestart(ctx, kcli) {
		if err := s.performPreUpgradeStatefulSetRestart(ctx, kcli, logf); err != nil {
			return errors.Wrap(err, "pre-upgrade statefulset restart")
		}
	}

	// Now perform the helm upgrade (normal path for both cases)
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

	return nil
}

// needsScalingRestart checks if this upgrade requires SeaweedFS master pod restart
// due to scaling from single replica to HA mode from versions < 2.7.3
func needsScalingRestart(ctx context.Context, kcli client.Client) bool {
	logrus.Info("Checking if scaling fix is needed for upgrade from pre-2.7.3")

	// Get the latest installation to use for getting the previous one
	latest, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		logrus.Infof("Could not get latest installation: %v", err)
		return false
	}

	// Get the previous installation to check version
	previous, err := kubeutils.GetPreviousInstallation(ctx, kcli, latest)
	if err != nil {
		logrus.Infof("Could not get previous installation: %v", err)
		return false
	}

	if previous == nil || previous.Spec.Config == nil || previous.Spec.Config.Version == "" {
		logrus.Info("Previous installation has no version config, skipping scaling fix")
		return false
	}

	// Parse previous version
	prevVersion, err := semver.NewVersion(previous.Spec.Config.Version)
	if err != nil {
		logrus.Infof("Could not parse previous version %s: %v", previous.Spec.Config.Version, err)
		return false
	}

	// Only restart if upgrading from < 2.7.3
	if !lessThanECVersion273(prevVersion) {
		logrus.Infof("Previous version %s >= 2.7.3, no scaling fix needed", prevVersion.String())
		return false
	}
	logrus.Infof("Previous version %s < 2.7.3, checking StatefulSet configuration", prevVersion.String())

	// Check if SeaweedFS StatefulSet exists and check current replica configuration
	var sts appsv1.StatefulSet
	nsn := client.ObjectKey{Namespace: "seaweedfs", Name: "seaweedfs-master"}
	if err := kcli.Get(ctx, nsn, &sts); err != nil {
		logrus.Infof("Could not get SeaweedFS master StatefulSet: %v", err)
		return false
	}

	// Check current replica configuration - need scaling fix if:
	// - Currently has 1 replica (normal single-node mode from pre-2.7.3)
	// - Already scaled to 3 but not all ready (previous upgrade attempt potentially failed)
	currentReplicas := int32(1) // default replica count
	if sts.Spec.Replicas != nil {
		currentReplicas = *sts.Spec.Replicas
	}
	logrus.Infof("StatefulSet current replicas: %d, ready replicas: %d", currentReplicas, sts.Status.ReadyReplicas)

	if currentReplicas == 1 {
		logrus.Info("Scaling fix needed - currently 1 replica, upgrading from pre-2.7.3")
		return true
	}

	if currentReplicas == 3 && sts.Status.ReadyReplicas < 3 {
		logrus.Info("Scaling fix needed - 3 replicas configured but not all ready")
		return true
	}

	logrus.Infof("No scaling fix needed - StatefulSet has %d replicas with %d ready", currentReplicas, sts.Status.ReadyReplicas)
	return false
}

// lessThanECVersion273 checks if a version is less than 2.7.3
func lessThanECVersion273(ver *semver.Version) bool {
	version273 := semver.MustParse("2.7.3")
	return ver.LessThan(version273)
}

// scaleStatefulSet directly scales the StatefulSet to the target replica count
func (s *SeaweedFS) scaleStatefulSet(ctx context.Context, kcli client.Client, replicas int32) error {
	// Get the current StatefulSet
	var sts appsv1.StatefulSet
	nsn := client.ObjectKey{Namespace: "seaweedfs", Name: "seaweedfs-master"}
	if err := kcli.Get(ctx, nsn, &sts); err != nil {
		return errors.Wrap(err, "get SeaweedFS master StatefulSet")
	}

	// Update replica count if needed
	if sts.Spec.Replicas == nil || *sts.Spec.Replicas != replicas {
		currentReplicas := int32(1)
		if sts.Spec.Replicas != nil {
			currentReplicas = *sts.Spec.Replicas
		}

		logrus.Infof("Scaling SeaweedFS master StatefulSet from %d to %d replicas", currentReplicas, replicas)

		sts.Spec.Replicas = &replicas
		if err := kcli.Update(ctx, &sts); err != nil {
			return errors.Wrap(err, "update StatefulSet replica count")
		}

	}

	return nil
}

// waitForStatefulSetScaleDown waits for all pods in the StatefulSet to be terminated
func (s *SeaweedFS) waitForStatefulSetScaleDown(ctx context.Context, kcli client.Client) error {
	// Wait for StatefulSet to scale down completely
	kubeutils := &kubeutils.KubeUtils{}
	if err := kubeutils.WaitForStatefulset(ctx, kcli, "seaweedfs", "seaweedfs-master", nil); err != nil {
		// This might fail because we're scaling to 0, continue to verify pods are terminated
		logrus.Infof("WaitForStatefulset returned error (expected when scaling to 0): %v", err)
	}

	// Retry logic: verify no pods are running by checking for remaining pods
	for i := 0; i < 5; i++ {
		var podList corev1.PodList
		if err := kcli.List(ctx, &podList, client.InNamespace("seaweedfs"), client.MatchingLabels{
			"app.kubernetes.io/name":      "seaweedfs",
			"app.kubernetes.io/component": "master",
		}); err != nil {
			if apierrors.IsNotFound(err) {
				break // No pods found, we're done
			}
			return errors.Wrap(err, "list seaweedfs master pods")
		}

		// Count running/pending pods
		runningPods := 0
		for _, pod := range podList.Items {
			if pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending {
				runningPods++
			}
		}

		if runningPods == 0 {
			break // All pods terminated
		}

		if i == 4 { // Last attempt
			return fmt.Errorf("expected 0 running pods after %d retries, but found %d pods still running/pending", i+1, runningPods)
		}

		// Wait before retry
		logrus.Infof("Still waiting for %d pods to terminate, retrying in 10 seconds...", runningPods)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
			continue
		}
	}

	logrus.Info("All SeaweedFS master pods have been terminated successfully")
	return nil
}

// performPreUpgradeStatefulSetRestart handles scaling fix for pre-2.7.3 upgrades by scaling down to 0,
// waiting for termination, then allowing helm upgrade to scale back to 3 with correct configuration.
func (s *SeaweedFS) performPreUpgradeStatefulSetRestart(ctx context.Context, kcli client.Client, logf types.LogFunc) error {
	logf("Detected scaling upgrade from pre-2.7.3, performing safe scaling fix before helm upgrade")

	// Create timeout context for scaling operations (5 minutes)
	scaleCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if err := s.scaleStatefulSet(scaleCtx, kcli, 0); err != nil {
		return errors.Wrap(err, "failed to scale StatefulSet down to 0")
	}

	if err := s.waitForStatefulSetScaleDown(scaleCtx, kcli); err != nil {
		return errors.Wrap(err, "failed to wait for pods to terminate")
	}

	logf("Scaling fix complete, proceeding with helm upgrade (will scale back to 3)")
	return nil
}
