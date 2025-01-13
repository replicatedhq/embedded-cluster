package migratev2

import (
	"context"
	"fmt"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// copyInstallationsToConfigMaps copies the Installation CRs to ConfigMaps.
func copyInstallationsToConfigMaps(ctx context.Context, logf LogFunc, cli client.Client) error {
	var installationList ecv1beta1.InstallationList
	err := cli.List(ctx, &installationList)
	if err != nil {
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
	in.Spec.SourceType = ecv1beta1.InstallationSourceTypeConfigMap
	err := kubeutils.CreateInstallation(ctx, cli, in)
	if !k8serrors.IsAlreadyExists(err) {
		err := kubeutils.UpdateInstallation(ctx, cli, in)
		if err != nil {
			return fmt.Errorf("update installation: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("create installation: %w", err)
	}
	return nil
}

// NOTE: this only deletes the Installation CRs, not the CRD itself.
func deleteInstallationCRs(ctx context.Context, cli client.Client) error {
	var installations ecv1beta1.InstallationList
	err := cli.List(ctx, &installations)
	if err != nil {
		return fmt.Errorf("list installations: %w", err)
	}

	for _, installation := range installations.Items {
		err := cli.Delete(ctx, &installation)
		if err != nil {
			return fmt.Errorf("delete installation %s: %w", installation.Name, err)
		}
	}

	return nil
}
