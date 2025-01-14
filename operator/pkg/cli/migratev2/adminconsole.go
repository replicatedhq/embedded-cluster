package migratev2

import (
	"context"
	"fmt"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// enableV2AdminConsole enables Embedded Cluster V2 in kotsadm by setting the IS_EC2_INSTALL
// environment variable to true in the admin-console chart and waits for the chart to be updated.
func enableV2AdminConsole(ctx context.Context, logf LogFunc, cli client.Client, in *ecv1beta1.Installation) error {
	logf("Updating admin-console chart values")
	err := updateAdminConsoleClusterConfig(ctx, cli)
	if err != nil {
		return fmt.Errorf("update cluster config: %w", err)
	}
	logf("Successfully updated admin-console chart values")

	logf("Setting v2 installation status to true")
	err = setIsEC2InstallInstallationStatus(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("set isEC2Install install status: %w", err)
	}
	logf("Successfully set v2 installation status to true")

	logf("Waiting for admin-console deployment to be updated")
	err = waitForAdminConsoleDeployment(ctx, cli)
	if err != nil {
		return fmt.Errorf("wait for chart update: %w", err)
	}
	logf("Successfully waited for admin-console deployment to be updated")

	return nil
}

func updateAdminConsoleClusterConfig(ctx context.Context, cli client.Client) error {
	var clusterConfig k0sv1beta1.ClusterConfig
	nsn := apitypes.NamespacedName{Name: "k0s", Namespace: "kube-system"}
	err := cli.Get(ctx, nsn, &clusterConfig)
	if err != nil {
		return fmt.Errorf("get k0s cluster config: %w", err)
	}

	for ix, ext := range clusterConfig.Spec.Extensions.Helm.Charts {
		if ext.Name == "admin-console" {
			values, err := updateAdminConsoleChartValues([]byte(ext.Values))
			if err != nil {
				return fmt.Errorf("update admin-console chart values: %w", err)
			}
			ext.Values = string(values)

			clusterConfig.Spec.Extensions.Helm.Charts[ix] = ext
		}
	}

	err = cli.Update(ctx, &clusterConfig)
	if err != nil {
		return fmt.Errorf("update k0s cluster config: %w", err)
	}

	return nil
}

func updateAdminConsoleChartValues(values []byte) ([]byte, error) {
	var m map[string]interface{}
	err := yaml.Unmarshal(values, &m)
	if err != nil {
		return nil, fmt.Errorf("unmarshal values: %w", err)
	}

	m["isEC2Install"] = "true"

	b, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal values: %w", err)
	}

	return b, nil
}

// setIsEC2InstallInstallationStatus is needed to inform the operator that the installation has
// been upgraded to v2 which prevents the operator from reconciling the installation.
func setIsEC2InstallInstallationStatus(ctx context.Context, cli client.Client, in *ecv1beta1.Installation) error {
	copy := in.DeepCopy()
	copy.Spec.SourceType = ecv1beta1.InstallationSourceTypeCRD

	if copy.Status.Conditions == nil {
		copy.Status.Conditions = []metav1.Condition{}
	}
	copy.Status.SetCondition(metav1.Condition{
		Type:               ConditionTypeIsEC2Install,
		Status:             metav1.ConditionTrue,
		Reason:             "MigrationComplete",
		ObservedGeneration: copy.Generation,
	})

	err := kubeutils.UpdateInstallationStatus(ctx, cli, copy)
	if err != nil {
		return fmt.Errorf("update installation: %w", err)
	}

	return nil
}

func waitForAdminConsoleDeployment(ctx context.Context, cli client.Client) error {
	err := kubeutils.WaitForDeployment(ctx, cli, "kotsadm", "kotsadm")
	if err != nil {
		return fmt.Errorf("wait for kotsadm deployment: %w", err)
	}
	return nil
}
