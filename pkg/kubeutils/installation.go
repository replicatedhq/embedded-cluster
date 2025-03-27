package kubeutils

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/Masterminds/semver/v3"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ErrNoInstallations struct{}

func (e ErrNoInstallations) Error() string {
	return "no installations found"
}

type ErrInstallationNotFound struct{}

func (e ErrInstallationNotFound) Error() string {
	return "installation not found"
}

func (k *KubeUtils) WaitForInstallation(ctx context.Context, cli client.Client, writer *spinner.MessageWriter) error {
	backoff := wait.Backoff{Steps: 60 * 5, Duration: time.Second, Factor: 1.0, Jitter: 0.1}
	var lasterr error

	if err := wait.ExponentialBackoffWithContext(
		ctx, backoff, func(ctx context.Context) (bool, error) {
			lastInstall, err := GetLatestInstallation(ctx, cli)
			if err != nil {
				lasterr = fmt.Errorf("unable to get latest installation: %v", err)
				return false, nil
			}

			if writer != nil {
				writeInstallationStatusMessage(writer, lastInstall)
			}

			// check the status of the installation
			if lastInstall.Status.State == ecv1beta1.InstallationStateInstalled {
				return true, nil
			}
			lasterr = fmt.Errorf("installation state is %q (%q)", lastInstall.Status.State, lastInstall.Status.Reason)

			if lastInstall.Status.State == ecv1beta1.InstallationStateFailed {
				return false, fmt.Errorf("installation failed: %s", lastInstall.Status.Reason)
			}

			if lastInstall.Status.State == ecv1beta1.InstallationStateHelmChartUpdateFailure {
				return false, fmt.Errorf("helm chart installation failed: %s", lastInstall.Status.Reason)
			}

			return false, nil
		},
	); err != nil {
		if wait.Interrupted(err) {
			if errors.Is(err, context.Canceled) {
				if lasterr != nil {
					err = errors.Join(err, lasterr)
				}
				return err
			} else if lasterr != nil {
				return fmt.Errorf("timed out waiting for the installation to finish: %v", lasterr)
			} else {
				return fmt.Errorf("timed out waiting for the installation to finish")
			}
		}
		return fmt.Errorf("error waiting for installation: %v", err)
	}
	return nil
}

func writeInstallationStatusMessage(writer *spinner.MessageWriter, install *ecv1beta1.Installation) {
	if install.Status.State != ecv1beta1.InstallationStatePendingChartCreation {
		return
	}

	if install.Spec.Config == nil || install.Spec.Config.Extensions.Helm == nil {
		return
	}
	numDesiredCharts := len(install.Spec.Config.Extensions.Helm.Charts)

	pendingChartsMap := map[string]struct{}{}
	for _, chartName := range install.Status.PendingCharts {
		pendingChartsMap[chartName] = struct{}{}
	}

	numPendingCharts := 0
	for _, ch := range install.Spec.Config.Extensions.Helm.Charts {
		if _, ok := pendingChartsMap[ch.Name]; ok {
			numPendingCharts++
		}
	}
	numCompletedCharts := numDesiredCharts - numPendingCharts

	if numCompletedCharts < numDesiredCharts {
		writer.Infof("Waiting for additional components to be ready (%d/%d)", numCompletedCharts, numDesiredCharts)
	} else {
		writer.Infof("Finalizing additional components")
	}
}

func CreateInstallation(ctx context.Context, cli client.Client, in *ecv1beta1.Installation) error {
	in.Spec.SourceType = ecv1beta1.InstallationSourceTypeCRD

	if in.ObjectMeta.Labels == nil {
		in.ObjectMeta.Labels = map[string]string{}
	}
	in.ObjectMeta.Labels["replicated.com/disaster-recovery"] = "ec-install"

	return cli.Create(ctx, in)
}

func UpdateInstallation(ctx context.Context, cli client.Client, in *ecv1beta1.Installation, mutate func(in *ecv1beta1.Installation)) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := cli.Get(ctx, client.ObjectKey{Namespace: in.Namespace, Name: in.Name}, in)
		if err != nil {
			return fmt.Errorf("get installation before updating: %w", err)
		}

		mutate(in)

		err = cli.Update(ctx, in)
		if err != nil {
			return fmt.Errorf("update installation: %w", err)
		}
		return nil
	})
}

// UpdateInstallationStatus updates the status of an installation.
// WARNING: this function updates the passed installation's _spec_ to match the spec in the cluster.
func UpdateInstallationStatus(ctx context.Context, cli client.Client, in *ecv1beta1.Installation, mutate func(status *ecv1beta1.InstallationStatus)) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := cli.Get(ctx, client.ObjectKey{Namespace: in.Namespace, Name: in.Name}, in)
		if err != nil {
			return fmt.Errorf("get installation before updating status: %w", err)
		}

		mutate(&in.Status)

		err = cli.Status().Update(ctx, in)
		if err != nil {
			return fmt.Errorf("update installation status: %w", err)
		}
		return nil
	})
}

func SetInstallationState(ctx context.Context, cli client.Client, in *ecv1beta1.Installation, state string, reason string, pendingCharts ...string) error {
	return UpdateInstallationStatus(ctx, cli, in, func(status *ecv1beta1.InstallationStatus) {
		status.SetState(state, reason, pendingCharts)
	})
}

func ListInstallations(ctx context.Context, cli client.Client) ([]ecv1beta1.Installation, error) {
	var list ecv1beta1.InstallationList
	err := cli.List(ctx, &list)
	if meta.IsNoMatchError(err) {
		// this will happen if the CRD is not yet installed
		return nil, ErrNoInstallations{}
	} else if err != nil {
		return nil, err
	}
	installs := list.Items
	sort.SliceStable(installs, func(i, j int) bool {
		return installs[j].Name < installs[i].Name
	})
	var previous *ecv1beta1.Installation
	for i := len(installs) - 1; i >= 0; i-- {
		install, didUpdate, err := MaybeOverrideInstallationDataDirs(installs[i], previous)
		if err != nil {
			return nil, fmt.Errorf("override installation data dirs: %w", err)
		}
		if didUpdate {
			runtimeConfig := install.Spec.RuntimeConfig
			err := UpdateInstallation(ctx, cli, &install, func(in *ecv1beta1.Installation) {
				in.Spec.RuntimeConfig = runtimeConfig
			})
			if err != nil {
				return nil, fmt.Errorf("update installation with legacy data dirs: %w", err)
			}
			log := ctrl.LoggerFrom(ctx)
			log.Info("Updated installation with legacy data dirs", "installation", install.Name)
		}
		installs[i] = install
		previous = &install
	}
	return installs, nil
}

func GetInstallation(ctx context.Context, cli client.Client, name string) (*ecv1beta1.Installation, error) {
	installations, err := ListInstallations(ctx, cli)
	if err != nil {
		return nil, err
	}
	if len(installations) == 0 {
		return nil, ErrNoInstallations{}
	}

	for _, installation := range installations {
		if installation.Name == name {
			return &installation, nil
		}
	}

	// if we get here, we didn't find the installation
	return nil, ErrInstallationNotFound{}
}

func GetLatestInstallation(ctx context.Context, cli client.Client) (*ecv1beta1.Installation, error) {
	installations, err := ListInstallations(ctx, cli)
	if err != nil {
		return nil, err
	}
	if len(installations) == 0 {
		return nil, ErrNoInstallations{}
	}

	// get the latest installation
	return &installations[0], nil
}

// GetPreviousInstallation returns the latest installation object in the cluster OTHER than the one passed as an argument.
func GetPreviousInstallation(ctx context.Context, cli client.Client, in *ecv1beta1.Installation) (*ecv1beta1.Installation, error) {
	installations, err := ListInstallations(ctx, cli)
	if err != nil {
		return nil, err
	}
	if len(installations) == 0 {
		return nil, ErrNoInstallations{}
	}

	// find the first installation with a different name than the one we're upgrading to
	for _, installation := range installations {
		if installation.Name != in.Name {
			return &installation, nil
		}
	}

	// if we get here, we didn't find a previous installation
	return nil, ErrInstallationNotFound{}
}

var (
	version115            = semver.MustParse("1.15.0")
	oldVersionSchemeRegex = regexp.MustCompile(`.*\+ec\.[0-9]+`)
)

func lessThanK0s115(ver *semver.Version) bool {
	if oldVersionSchemeRegex.MatchString(ver.Original()) {
		return true
	}
	return ver.LessThan(version115)
}

// MaybeOverrideInstallationDataDirs checks if the previous installation is less than 1.15.0 that
// didn't store the location of the data directories in the installation object. If it is not set,
// it will set the annotation and update the installation object with the old location of the data
// directories.
func MaybeOverrideInstallationDataDirs(in ecv1beta1.Installation, previous *ecv1beta1.Installation) (ecv1beta1.Installation, bool, error) {
	if previous != nil {
		ver, err := semver.NewVersion(previous.Spec.Config.Version)
		if err != nil {
			return in, false, fmt.Errorf("parse version: %w", err)
		}

		if lessThanK0s115(ver) {
			didUpdate := false

			if in.Spec.RuntimeConfig == nil {
				in.Spec.RuntimeConfig = &ecv1beta1.RuntimeConfigSpec{}
			}

			// In prior versions, the data directories are not a subdirectory of /var/lib/embedded-cluster.
			if in.Spec.RuntimeConfig.K0sDataDirOverride != "/var/lib/k0s" {
				in.Spec.RuntimeConfig.K0sDataDirOverride = "/var/lib/k0s"
				didUpdate = true
			}
			if in.Spec.RuntimeConfig.OpenEBSDataDirOverride != "/var/openebs" {
				in.Spec.RuntimeConfig.OpenEBSDataDirOverride = "/var/openebs"
				didUpdate = true
			}

			return in, didUpdate, nil
		}
	}

	return in, false, nil
}

func SetInstallationConditionStatus(ctx context.Context, cli client.Client, in *ecv1beta1.Installation, condition metav1.Condition) error {
	return UpdateInstallationStatus(ctx, cli, in, func(status *ecv1beta1.InstallationStatus) {
		status.SetCondition(condition)
	})
}

func CheckInstallationConditionStatus(inStat ecv1beta1.InstallationStatus, conditionName string) metav1.ConditionStatus {
	for _, cond := range inStat.Conditions {
		if cond.Type == conditionName {
			return cond.Status
		}
	}

	return ""
}
