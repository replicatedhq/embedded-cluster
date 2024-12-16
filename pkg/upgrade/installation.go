package upgrade

import (
	"context"
	"fmt"

	clusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// reApplyInstallation updates the installation spec to match what's in the configmap used by the upgrade job.
// This is required because the installation CRD may have been updated as part of this upgrade, and additional fields may be present now.
func reApplyInstallation(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation) error {
	existingInstallation, err := kubeutils.GetInstallation(ctx, cli, in.Name)
	if err != nil {
		return fmt.Errorf("get installation: %w", err)
	}

	existingInstallation.Spec = *in.Spec.DeepCopy() // copy the spec in, in case there were fields added to the spec
	err = kubeutils.UpdateInstallation(ctx, cli, existingInstallation)
	if err != nil {
		return fmt.Errorf("update installation: %w", err)
	}

	return nil
}

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
