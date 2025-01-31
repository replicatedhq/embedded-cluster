package extensions

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/k8sutil"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	actionInstall   = "Install"
	actionUpgrade   = "Upgrade"
	actionUninstall = "Uninstall"
)

func Upgrade(ctx context.Context, kcli client.Client, prev *ecv1beta1.Installation, in *ecv1beta1.Installation) error {
	airgapChartsPath := ""
	if in.Spec.AirGap {
		airgapChartsPath = runtimeconfig.EmbeddedClusterChartsSubDir()
	}

	hcli, err := helm.NewHelm(helm.HelmOptions{
		K0sVersion: versions.K0sVersion,
		AirgapPath: airgapChartsPath,
	})
	if err != nil {
		return errors.Wrap(err, "create helm client")
	}

	// add new helm repos
	if in.Spec.Config.Extensions.Helm != nil {
		if err := addRepos(hcli, in.Spec.Config.Extensions.Helm.Repositories); err != nil {
			return errors.Wrap(err, "add repos")
		}
	}

	// diff the extensions
	diffResult := diffExtensions(prev.Spec.Config.Extensions, in.Spec.Config.Extensions)

	// install added extensions
	for _, ext := range diffResult.Added {
		if err := handleExtension(ctx, hcli, kcli, in, ext, actionInstall); err != nil {
			return err
		}
	}

	// upgrade modified extensions
	for _, ext := range diffResult.Modified {
		if err := handleExtension(ctx, hcli, kcli, in, ext, actionUpgrade); err != nil {
			return err
		}
	}

	// uninstall removed extensions
	for _, ext := range diffResult.Removed {
		if err := handleExtension(ctx, hcli, kcli, in, ext, actionUninstall); err != nil {
			return err
		}
	}

	return nil
}

func handleExtension(ctx context.Context, hcli *helm.Helm, kcli client.Client, in *ecv1beta1.Installation, ext ecv1beta1.Chart, action string) (finalErr error) {
	// check if we already processed this extension
	conditionStatus, err := k8sutil.GetConditionStatus(ctx, kcli, in.Name, conditionName(ext))
	if err != nil {
		return errors.Wrap(err, "get condition status")
	}
	if conditionStatus == metav1.ConditionTrue {
		fmt.Printf("%s already %sed\n", ext.Name, action)
		return nil
	}

	actionIng := ""
	actionEd := ""
	if action == actionInstall || action == actionUninstall {
		actionIng = action + "ing"
		actionEd = action + "ed"
	} else if action == actionUpgrade {
		actionIng = "upgrading"
		actionEd = "upgraded"
	}
	fmt.Printf("%s %s\n", actionIng, ext.Name)

	// mark as processing
	if err := k8sutil.SetConditionStatus(ctx, kcli, in, metav1.Condition{
		Type:   conditionName(ext),
		Status: metav1.ConditionFalse,
		Reason: actionIng,
	}); err != nil {
		return errors.Wrap(err, "set condition status")
	}

	defer func() {
		if finalErr == nil {
			// mark as processed successfully
			if err := k8sutil.SetConditionStatus(ctx, kcli, in, metav1.Condition{
				Type:   conditionName(ext),
				Status: metav1.ConditionTrue,
				Reason: actionEd,
			}); err != nil {
				fmt.Printf("failed to set condition status: %v", err)
			}
		} else {
			// mark as failed
			if err := k8sutil.SetConditionStatus(ctx, kcli, in, metav1.Condition{
				Type:    conditionName(ext),
				Status:  metav1.ConditionFalse,
				Reason:  action + "Failed",
				Message: cleanErrorMessage(finalErr),
			}); err != nil {
				fmt.Printf("failed to set condition status: %v", err)
			}
		}
	}()

	switch action {
	case actionInstall:
		exists, err := hcli.ReleaseExists(ctx, ext.TargetNS, ext.Name)
		if err != nil {
			return errors.Wrap(err, "check if release exists")
		}
		if exists {
			fmt.Printf("%s already installed\n", ext.Name)
			return nil
		}
		if err := install(ctx, hcli, ext); err != nil {
			return errors.Wrapf(err, "install %s", ext.Name)
		}

	case actionUpgrade:
		if err := upgrade(ctx, hcli, ext); err != nil {
			return errors.Wrapf(err, "upgrade %s", ext.Name)
		}

	case actionUninstall:
		exists, err := hcli.ReleaseExists(ctx, ext.TargetNS, ext.Name)
		if err != nil {
			return errors.Wrap(err, "check if release exists")
		}
		if !exists {
			fmt.Printf("%s already uninstalled\n", ext.Name)
			return nil
		}
		if err := uninstall(ctx, hcli, ext); err != nil {
			return errors.Wrapf(err, "uninstall %s", ext.Name)
		}
	}

	fmt.Printf("%s %sed successfully\n", ext.Name, actionEd)

	return nil
}
