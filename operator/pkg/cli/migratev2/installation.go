package migratev2

import (
	"context"
	"fmt"
	"time"

	"github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// setV2MigrationInProgress sets the Installation condition to indicate that the v2 migration is in
// progress.
func setV2MigrationInProgress(ctx context.Context, logf LogFunc, cli client.Client, in *ecv1beta1.Installation) error {
	logf("Setting v2 migration in progress")

	err := setInstallationCondition(ctx, cli, in, metav1.Condition{
		Type:   ecv1beta1.ConditionTypeV2MigrationInProgress,
		Status: metav1.ConditionTrue,
		Reason: "V2MigrationInProgress",
	})
	if err != nil {
		return fmt.Errorf("set v2 migration in progress condition: %w", err)
	}

	logf("Successfully set v2 migration in progress")
	return nil
}

// waitForInstallationStateInstalled waits for the installation to be in a successful state and
// ready for the migration.
func waitForInstallationStateInstalled(ctx context.Context, logf LogFunc, cli client.Client, installation *ecv1beta1.Installation) error {
	logf("Waiting for installation to reconcile")

	err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		in, err := kubeutils.GetCRDInstallation(ctx, cli, installation.Name)
		if err != nil {
			return false, fmt.Errorf("get installation: %w", err)
		}

		switch in.Status.State {
		// Success states
		case ecv1beta1.InstallationStateInstalled, ecv1beta1.InstallationStateAddonsInstalled:
			return true, nil

		// Failure states
		case ecv1beta1.InstallationStateFailed, ecv1beta1.InstallationStateHelmChartUpdateFailure:
			return false, fmt.Errorf("installation failed: %s", in.Status.Reason)
		case ecv1beta1.InstallationStateObsolete:
			return false, fmt.Errorf("installation is obsolete")

		// In progress states
		default:
			return false, nil
		}
	})
	if err != nil {
		return err
	}

	logf("Installation reconciled")
	return nil
}

// copyInstallationsToConfigMaps copies the Installation CRs to ConfigMaps.
func copyInstallationsToConfigMaps(ctx context.Context, logf LogFunc, cli client.Client) error {
	var installationList ecv1beta1.InstallationList
	err := cli.List(ctx, &installationList)
	if err != nil {
		// handle the case where the CRD has already been uninstalled
		if meta.IsNoMatchError(err) {
			return nil
		}
		return fmt.Errorf("list installations: %w", err)
	}

	for _, installation := range installationList.Items {
		logf("Copying installation %s to config map", installation.Name)
		err := ensureInstallationConfigMap(ctx, cli, &installation)
		if err != nil {
			return fmt.Errorf("ensure config map for installation %s: %w", installation.Name, err)
		}
		logf("Successfully copied installation %s to config map", installation.Name)
	}

	return nil
}

func ensureInstallationConfigMap(ctx context.Context, cli client.Client, in *ecv1beta1.Installation) error {
	copy := in.DeepCopy()
	err := kubeutils.CreateInstallation(ctx, cli, copy)
	if k8serrors.IsAlreadyExists(err) {
		err := kubeutils.UpdateInstallation(ctx, cli, copy)
		if err != nil {
			return fmt.Errorf("update installation: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("create installation: %w", err)
	}
	return nil
}

// ensureInstallationStateInstalled sets the ConfigMap installation state to installed and updates
// the status to mark the upgrade as complete.
func ensureInstallationStateInstalled(ctx context.Context, logf LogFunc, cli client.Client, in *ecv1beta1.Installation) error {
	logf("Setting installation state to installed")

	// the installation will be in a ConfigMap at this point
	copy, err := kubeutils.GetInstallation(ctx, cli, in.Name)
	if err != nil {
		return fmt.Errorf("get installation: %w", err)
	}

	copy.Status.SetState(v1beta1.InstallationStateInstalled, "V2MigrationComplete", nil)
	meta.RemoveStatusCondition(&copy.Status.Conditions, ecv1beta1.ConditionTypeV2MigrationInProgress)
	meta.RemoveStatusCondition(&copy.Status.Conditions, ecv1beta1.ConditionTypeDisableReconcile)

	err = kubeutils.UpdateInstallationStatus(ctx, cli, copy)
	if err != nil {
		return fmt.Errorf("update installation status: %w", err)
	}

	logf("Successfully set installation state to installed")
	return nil
}

func setInstallationCondition(ctx context.Context, cli client.Client, in *ecv1beta1.Installation, condition metav1.Condition) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var copy ecv1beta1.Installation
		err := cli.Get(ctx, client.ObjectKey{Name: in.Name}, &copy)
		if err != nil {
			return fmt.Errorf("get installation: %w", err)
		}

		copy.Status.SetCondition(condition)

		err = cli.Status().Update(ctx, &copy)
		if err != nil {
			return fmt.Errorf("update installation status: %w", err)
		}

		return nil
	})
}
