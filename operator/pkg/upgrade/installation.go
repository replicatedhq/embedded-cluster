package upgrade

import (
	"context"
	"fmt"

	clusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateInstallation(ctx context.Context, cli client.Client, original *clusterv1beta1.Installation) error {
	log := controllerruntime.LoggerFrom(ctx)
	in := original.DeepCopy()

	// check if the installation already exists - this function can be called multiple times
	// if the installation is already created, we can just return
	nsn := types.NamespacedName{Name: in.Name}
	var existingInstallation clusterv1beta1.Installation
	if err := cli.Get(ctx, nsn, &existingInstallation); err == nil {
		log.Info(fmt.Sprintf("Installation %s already exists", in.Name))
		return nil
	}
	log.Info(fmt.Sprintf("Creating installation %s", in.Name))

	in, err := kubeutils.MaybeOverrideInstallationDataDirs(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("override installation data dirs: %w", err)
	}

	err = cli.Create(ctx, in)
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

// setInstallationState gets the installation object of the given name and sets the state to the given state.
func setInstallationState(ctx context.Context, cli client.Client, name string, state string, reason string, pendingCharts ...string) error {
	existingInstallation := &clusterv1beta1.Installation{}
	err := cli.Get(ctx, client.ObjectKey{Name: name}, existingInstallation)
	if err != nil {
		return fmt.Errorf("get installation: %w", err)
	}
	existingInstallation.Status.SetState(state, reason, pendingCharts)
	err = cli.Status().Update(ctx, existingInstallation)
	if err != nil {
		return fmt.Errorf("update installation status: %w", err)
	}
	return nil
}

// reApplyInstallation updates the installation spec to match what's in the configmap used by the upgrade job.
// This is required because the installation CRD may have been updated as part of this upgrade, and additional fields may be present now.
func reApplyInstallation(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation) error {
	existingInstallation := &clusterv1beta1.Installation{}
	err := cli.Get(ctx, client.ObjectKey{Name: in.Name}, existingInstallation)
	if err != nil {
		return fmt.Errorf("get installation: %w", err)
	}

	existingInstallation.Spec = *in.Spec.DeepCopy() // copy the spec in, in case there were fields added to the spec
	err = cli.Update(ctx, existingInstallation)
	if err != nil {
		return fmt.Errorf("update installation: %w", err)
	}

	return nil
}
