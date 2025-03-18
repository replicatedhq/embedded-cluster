package util

import (
	"context"
	"fmt"
	"strings"

	"github.com/replicatedhq/embedded-cluster/operator/charts"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

func UpgradeInstallationCRD(ctx context.Context, kcli client.Client) error {
	// decode the CRD file
	crds := strings.Split(charts.InstallationCRDFile, "\n---\n")

	for _, crdYaml := range crds {
		var crd v1.CustomResourceDefinition
		if err := yaml.Unmarshal([]byte(crdYaml), &crd); err != nil {
			return fmt.Errorf("unmarshal installation CRD: %w", err)
		}

		// get the CRD from the cluster
		var existingCrd v1.CustomResourceDefinition
		if err := kcli.Get(ctx, client.ObjectKey{Name: crd.Name}, &existingCrd); err != nil {
			return fmt.Errorf("get installation CRD: %w", err)
		}

		// update the existing CRD spec to match the new CRD spec
		existingCrd.Spec = crd.Spec
		if err := kcli.Update(ctx, &existingCrd); err != nil {
			return fmt.Errorf("update installation CRD: %w", err)
		}

		// wait for the CRD to be ready
		if err := kubeutils.WaitForCRDToBeReady(ctx, kcli, crd.Name); err != nil {
			return fmt.Errorf("wait for installation CRD to be ready: %w", err)
		}
	}

	return nil
}
