package upgrade

import (
	"context"
	"fmt"
	"log/slog"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateInstallation(ctx context.Context, cli client.Client, original *ecv1beta1.Installation) error {
	in := original.DeepCopy()

	// check if the installation already exists - this function can be called multiple times
	// if the installation is already created, we can just return
	if in, err := kubeutils.GetInstallation(ctx, cli, in.Name); err == nil {
		slog.Info("Installation already exists", "name", in.Name)
		return nil
	}

	err := kubeutils.EnsureInstallationCRD(ctx, cli)
	if err != nil {
		return fmt.Errorf("upgrade installation CRD: %w", err)
	}

	slog.Info("Creating installation", "name", in.Name)

	err = kubeutils.CreateInstallation(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("create installation: %w", err)
	}

	err = kubeutils.SetInstallationState(ctx, cli, in, ecv1beta1.InstallationStateInstalling, "Upgrading Kubernetes", "")
	if err != nil {
		return fmt.Errorf("update installation status: %w", err)
	}

	if err := disableOldInstallations(ctx, cli); err != nil {
		// don't fail the upgrade if we can't disable old installations
		// as this is not a critical operation
		slog.Error("Failed to disable old installations", "error", err)
	}

	slog.Info("Installation created", "name", in.Name)

	return nil
}

// disableOldInstallations resets old installation statuses keeping only the newest one with
// proper status set. It sets the state for all old installations as "obsolete" as they
// are not necessary anymore and are kept only for historic reasons.
func disableOldInstallations(ctx context.Context, cli client.Client) error {
	ins, err := kubeutils.ListInstallations(ctx, cli)
	if err != nil {
		return fmt.Errorf("list installations: %w", err)
	}

	for _, in := range ins[1:] {
		if in.Status.State == ecv1beta1.InstallationStateObsolete {
			continue
		}

		err := kubeutils.UpdateInstallationStatus(ctx, cli, &in, func(status *ecv1beta1.InstallationStatus) {
			status.NodesStatus = nil
			status.SetState(ecv1beta1.InstallationStateObsolete, "This is not the most recent installation object", nil)
		})
		if err != nil {
			return fmt.Errorf("update installation: %w", err)
		}
	}

	return nil
}
