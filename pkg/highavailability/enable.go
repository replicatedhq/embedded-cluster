package highavailability

import (
	"context"
	"fmt"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg/constants"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

// CanEnableHA checks if high availability can be enabled in the cluster.
func CanEnableHA(ctx context.Context, kcli client.Client) (bool, error) {
	installation, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return false, fmt.Errorf("unable to get latest installation: %w", err)
	}
	if installation.Spec.HighAvailability {
		return false, nil
	}
	if err := kcli.Get(ctx, types.NamespacedName{Name: constants.EcRestoreStateCMName, Namespace: "embedded-cluster"}, &v1.ConfigMap{}); err == nil {
		return false, nil // cannot enable HA during a restore
	} else if !errors.IsNotFound(err) {
		return false, fmt.Errorf("unable to get restore state configmap: %w", err)
	}
	ncps, err := kubeutils.NumOfControlPlaneNodes(ctx, kcli)
	if err != nil {
		return false, fmt.Errorf("unable to check control plane nodes: %w", err)
	}
	return ncps >= 3, nil
}

// EnableHA enables high availability in the installation object
// and waits for the migration to be complete.
func EnableHA(ctx context.Context, kcli client.Client) error {
	loading := spinner.Start()
	defer loading.Close()
	loading.Infof("Enabling high availability")
	in, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return fmt.Errorf("unable to get latest installation: %w", err)
	}
	if !in.Spec.HighAvailability {
		// only update the installation/create seaweed service if HA is not already enabled
		if err := createSeaweedfsResources(ctx, kcli, in); err != nil {
			return fmt.Errorf("unable to create seaweedfs resources: %w", err)
		}

		in.Spec.HighAvailability = true
		if err := kcli.Update(ctx, in); err != nil {
			return fmt.Errorf("unable to update installation: %w", err)
		}
	}

	if err := kubeutils.WaitForHAInstallation(ctx, kcli); err != nil {
		return fmt.Errorf("unable to wait for ha installation: %w", err)
	}
	loading.Infof("High availability enabled!")
	return nil
}
