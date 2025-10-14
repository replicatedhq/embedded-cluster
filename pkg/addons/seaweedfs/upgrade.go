package seaweedfs

import (
	"context"
	"fmt"
	"strings"
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
		return fmt.Errorf("checking release existence: %w", err)
	}
	if !exists {
		logrus.Debugf("Release not found, installing release %s in namespace %s", s.ReleaseName(), s.Namespace())
		return s.Install(ctx, logf, kcli, mcli, hcli, domains, overrides)
	}

	if err := s.ensurePreRequisites(ctx, kcli); err != nil {
		return fmt.Errorf("creating prerequisites: %w", err)
	}

	values, err := s.GenerateHelmValues(ctx, kcli, domains, overrides)
	if err != nil {
		return fmt.Errorf("generating helm values: %w", err)
	}

	needsRestart, err := s.needsScalingRestart(ctx, kcli)
	if err != nil {
		return fmt.Errorf("checking scaling restart need: %w", err)
	}
	if needsRestart {
		if err := s.performPreUpgradeStatefulSetRestart(ctx, kcli, logf); err != nil {
			return fmt.Errorf("pre-upgrade statefulset restart: %w", err)
		}
	}

	// When upgrading from a previous version, we need to disable hashicorp raft as a rolling
	// update will fail if toggling raft implementation.
	shouldDisableHashicorpRaft, err := s.shouldDisableRaftHashicorp(ctx, kcli)
	if err != nil {
		return fmt.Errorf("checking if raft hashicorp should be disabled: %w", err)
	}
	if shouldDisableHashicorpRaft {
		logrus.Debug("Setting master.raftHashicorp=false and master.raftBootstrap=false")
		if err := helm.SetValue(values, "master.raftHashicorp", false); err != nil {
			return fmt.Errorf("setting master.raftHashicorp: %w", err)
		}
		if err := helm.SetValue(values, "master.raftBootstrap", false); err != nil {
			return fmt.Errorf("setting master.raftBootstrap: %w", err)
		}
		logrus.Debug("master.raftHashicorp=false and master.raftBootstrap=false set")
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
		return fmt.Errorf("helm upgrade: %w", err)
	}

	return nil
}

// needsScalingRestart checks if this upgrade requires SeaweedFS master pod restart
// due to scaling from single replica to HA mode from versions < 2.7.3
func (s *SeaweedFS) needsScalingRestart(ctx context.Context, kcli client.Client) (bool, error) {
	logrus.Debug("Checking if scaling fix is needed for upgrade from pre-2.7.3")

	prevVersion, err := getPreviousECVersion(ctx, kcli)
	if err != nil {
		return false, fmt.Errorf("get previous installation: %w", err)
	} else if prevVersion == nil {
		logrus.Debug("No previous version found, no scaling fix needed")
		return false, nil
	}

	// Only restart if upgrading from < 2.7.3
	if !lessThanECVersion273(prevVersion) {
		logrus.Debugf("Previous version %s >= 2.7.3, no scaling fix needed", prevVersion)
		return false, nil
	}
	logrus.Debugf("Previous version %s < 2.7.3, checking StatefulSet configuration", prevVersion)

	// Check if SeaweedFS StatefulSet exists and check current replica configuration
	var sts appsv1.StatefulSet
	nsn := client.ObjectKey{Namespace: s.Namespace(), Name: "seaweedfs-master"}
	if err := kcli.Get(ctx, nsn, &sts); err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Debug("SeaweedFS master StatefulSet not found, no scaling fix needed")
			return false, nil // No StatefulSet means SeaweedFS not yet installed
		}
		return false, fmt.Errorf("getting SeaweedFS master StatefulSet: %w", err)
	}

	// Check current replica configuration - need scaling fix if:
	// - Currently has 1 replica (normal single-node mode from pre-2.7.3)
	// - Already scaled to 3 but not all ready (previous upgrade attempt potentially failed)
	currentReplicas := int32(1) // default replica count
	if sts.Spec.Replicas != nil {
		currentReplicas = *sts.Spec.Replicas
	}
	logrus.Debugf("StatefulSet current replicas: %d, ready replicas: %d", currentReplicas, sts.Status.ReadyReplicas)

	if currentReplicas == 1 {
		logrus.Debug("Scaling fix needed - currently 1 replica, upgrading from pre-2.7.3")
		return true, nil
	}

	if currentReplicas == 3 && sts.Status.ReadyReplicas < 3 {
		logrus.Debug("Scaling fix needed - 3 replicas configured but not all ready")
		return true, nil
	}

	logrus.Debugf("No scaling fix needed - StatefulSet has %d replicas with %d ready", currentReplicas, sts.Status.ReadyReplicas)
	return false, nil
}

func getPreviousECVersion(ctx context.Context, kcli client.Client) (*semver.Version, error) {
	latest, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return nil, fmt.Errorf("get latest installation: %w", err)
	}
	previous, err := kubeutils.GetPreviousInstallation(ctx, kcli, latest)
	if err != nil {
		var errNotFound kubeutils.ErrInstallationNotFound
		if errors.As(err, &errNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get previous installation: %w", err)
	}
	if previous.Spec.Config == nil || previous.Spec.Config.Version == "" {
		return nil, errors.New("previous installation has no version config")
	}
	sv, err := semver.NewVersion(previous.Spec.Config.Version)
	if err != nil {
		return nil, fmt.Errorf("parse previous version %s: %w", previous.Spec.Config.Version, err)
	}
	return sv, nil
}

var version273 = semver.MustParse("2.7.3")

// lessThanECVersion273 checks if a version is less than 2.7.3
func lessThanECVersion273(ver *semver.Version) bool {
	return ver.LessThan(version273)
}

// shouldDisableRaftHashicorp checks to see if there is a previous statefulset without
// -raftHashicorp argument
func (s *SeaweedFS) shouldDisableRaftHashicorp(ctx context.Context, kcli client.Client) (bool, error) {
	logrus.Debug("Checking if hashicorp raft should be disabled")

	var sts appsv1.StatefulSet
	nsn := client.ObjectKey{Namespace: s.Namespace(), Name: "seaweedfs-master"}
	if err := kcli.Get(ctx, nsn, &sts); client.IgnoreNotFound(err) != nil {
		return false, fmt.Errorf("get seaweedfs master statefulset: %w", err)
	} else if err != nil {
		// not found, so no previous statefulset
		logrus.Debug("No previous statefulset found, do not disable raft hashicorp")
		return false, nil
	}
	// check if the seaweedfs container has the -raftHashicorp argument
	for _, container := range sts.Spec.Template.Spec.Containers {
		if container.Name == "seaweedfs" {
			for _, arg := range container.Args {
				if strings.Contains(arg, "-raftHashicorp") {
					logrus.Debug("Raft hashicorp is enabled, do not disable it")
					return false, nil
				}
			}
		}
	}
	logrus.Debug("Raft hashicorp is disabled, disable it")
	return true, nil
}

// scaleStatefulSet directly scales the StatefulSet to the target replica count
func (s *SeaweedFS) scaleStatefulSet(ctx context.Context, kcli client.Client, replicas int32) error {
	// Get the current StatefulSet
	var sts appsv1.StatefulSet
	nsn := client.ObjectKey{Namespace: s.Namespace(), Name: "seaweedfs-master"}
	if err := kcli.Get(ctx, nsn, &sts); err != nil {
		return fmt.Errorf("getting SeaweedFS master StatefulSet: %w", err)
	}

	// Update replica count if needed
	if sts.Spec.Replicas == nil || *sts.Spec.Replicas != replicas {
		currentReplicas := int32(1)
		if sts.Spec.Replicas != nil {
			currentReplicas = *sts.Spec.Replicas
		}

		logrus.Debugf("Scaling SeaweedFS master StatefulSet from %d to %d replicas", currentReplicas, replicas)

		sts.Spec.Replicas = &replicas
		if err := kcli.Update(ctx, &sts); err != nil {
			return fmt.Errorf("updating StatefulSet replica count: %w", err)
		}

	}

	return nil
}

// waitForStatefulSetScaleDown waits for all pods in the StatefulSet to be terminated
func (s *SeaweedFS) waitForStatefulSetScaleDown(ctx context.Context, kcli client.Client) error {
	// Wait for StatefulSet to scale down completely
	if err := kubeutils.WaitForStatefulset(ctx, kcli, s.Namespace(), "seaweedfs-master", nil); err != nil {
		// This might fail because we're scaling to 0, continue to verify pods are terminated
		logrus.Debugf("WaitForStatefulset returned error (expected when scaling to 0): %v", err)
	}

	// Retry logic: verify no pods are running by checking for remaining pods
	for i := 0; i < 5; i++ {
		var podList corev1.PodList
		if err := kcli.List(ctx, &podList, client.InNamespace(s.Namespace()), client.MatchingLabels{
			"app.kubernetes.io/name":      "seaweedfs",
			"app.kubernetes.io/component": "master",
		}); err != nil {
			return fmt.Errorf("listing seaweedfs master pods: %w", err)
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
		logrus.Debugf("Still waiting for %d pods to terminate, retrying in 10 seconds...", runningPods)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
			continue
		}
	}

	logrus.Debug("All SeaweedFS master pods have been terminated successfully")
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
		return fmt.Errorf("scaling StatefulSet down to 0: %w", err)
	}

	if err := s.waitForStatefulSetScaleDown(scaleCtx, kcli); err != nil {
		return fmt.Errorf("waiting for pods to terminate: %w", err)
	}

	logf("Scaling fix complete, proceeding with helm upgrade (will scale back to 3)")
	return nil
}
