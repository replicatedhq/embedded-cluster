package upgrade

import (
	"context"
	"fmt"

	clusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"k8s.io/client-go/util/retry"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// setInstallationState gets the installation object of the given name and sets the state to the given state.
func setInstallationState(ctx context.Context, cli client.Client, name string, state string, reason string, pendingCharts ...string) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		existingInstallation := &clusterv1beta1.Installation{}
		err := cli.Get(ctx, client.ObjectKey{Name: name}, existingInstallation)
		if err != nil {
			return fmt.Errorf("get installation: %w", err)
		}
		existingInstallation.Status.SetState(state, reason, pendingCharts)
		err = kubeutils.UpdateInstallationStatus(ctx, cli, existingInstallation)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("persistent conflict error, failed to update installation %s status: %w", name, err)
	}

	return nil
}

func createInstallation(ctx context.Context, cli client.Client, original *clusterv1beta1.Installation) error {
	log := controllerruntime.LoggerFrom(ctx)
	in := original.DeepCopy()

	// check if the installation already exists - this function can be called multiple times
	// if the installation is already created, we can just return
	if in, err := kubeutils.GetInstallation(ctx, cli, in.Name); err == nil {
		log.Info(fmt.Sprintf("Installation %s already exists", in.Name))
		return nil
	}
	log.Info(fmt.Sprintf("Creating installation %s", in.Name))

	err := cli.Create(ctx, in)
	if err != nil {
		return fmt.Errorf("create installation: %w", err)
	}

	err = setInstallationState(ctx, cli, in.Name, clusterv1beta1.InstallationStateInstalling, "Upgrading Kubernetes via job", "")
	if err != nil {
		return fmt.Errorf("update installation status: %w", err)
	}

	log.Info("Installation created")

	return nil
}
