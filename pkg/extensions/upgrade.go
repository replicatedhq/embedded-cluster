package extensions

import (
	"context"
	"log/slog"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	actionInstall   = helmAction("Install")
	actionUpgrade   = helmAction("Upgrade")
	actionUninstall = helmAction("Uninstall")
	actionNoChange  = helmAction("NoChange")
)

type helmAction string

func Upgrade(ctx context.Context, kcli client.Client, hcli helm.Client, prev *ecv1beta1.Installation, in *ecv1beta1.Installation) error {
	// add new helm repos
	if in.Spec.Config.Extensions.Helm != nil {
		if err := addRepos(hcli, in.Spec.Config.Extensions.Helm.Repositories); err != nil {
			return errors.Wrap(err, "add repos")
		}
	}

	// diff the extensions
	var inExts, prevExts ecv1beta1.Extensions
	if in != nil && in.Spec.Config != nil {
		inExts = in.Spec.Config.Extensions
	}
	if prev != nil && prev.Spec.Config != nil {
		prevExts = prev.Spec.Config.Extensions
	}

	results := diffExtensions(prevExts, inExts)

	// first uninstall removed extensions in reverse order
	for i := len(results) - 1; i >= 0; i-- {
		result := results[i]
		if result.Action == actionUninstall {
			if err := handleExtensionUninstall(ctx, kcli, hcli, in, result.Ext); err != nil {
				return errors.Wrapf(err, "uninstall extension %s", result.Ext.Name)
			}
		}
	}

	// then install and upgrade modified extensions in order
	for _, result := range results {
		switch result.Action {
		case actionInstall:
			if err := handleExtensionInstall(ctx, kcli, hcli, in, result.Ext); err != nil {
				return errors.Wrapf(err, "install extension %s", result.Ext.Name)
			}
		case actionUpgrade:
			if err := handleExtensionUpgrade(ctx, kcli, hcli, in, result.Ext); err != nil {
				return errors.Wrapf(err, "upgrade extension %s", result.Ext.Name)
			}
		case actionNoChange:
			if err := handleExtensionNoop(ctx, kcli, in, result.Ext); err != nil {
				return errors.Wrapf(err, "noop extension %s", result.Ext.Name)
			}
		case actionUninstall:
			continue
		}
	}

	return nil
}

func handleExtensionInstall(ctx context.Context, kcli client.Client, hcli helm.Client, in *ecv1beta1.Installation, ext ecv1beta1.Chart) error {
	return handleExtension(ctx, kcli, in, ext, actionInstall, func() error {
		exists, err := hcli.ReleaseExists(ctx, ext.TargetNS, ext.Name)
		if err != nil {
			return errors.Wrap(err, "check if release exists")
		}
		if exists {
			slog.Info("Extension already installed", "name", ext.Name)
			return nil
		}
		if err := install(ctx, hcli, ext); err != nil {
			return errors.Wrap(err, "install")
		}
		return nil
	})
}

func handleExtensionUpgrade(ctx context.Context, kcli client.Client, hcli helm.Client, in *ecv1beta1.Installation, ext ecv1beta1.Chart) error {
	return handleExtension(ctx, kcli, in, ext, actionUpgrade, func() error {
		if err := upgrade(ctx, hcli, ext); err != nil {
			return errors.Wrap(err, "upgrade")
		}
		return nil
	})
}

func handleExtensionNoop(ctx context.Context, kcli client.Client, in *ecv1beta1.Installation, ext ecv1beta1.Chart) error {
	return handleExtension(ctx, kcli, in, ext, actionUpgrade, func() error {
		slog.Info("Extension is up to date", "name", ext.Name)
		return nil
	})
}

func handleExtensionUninstall(ctx context.Context, kcli client.Client, hcli helm.Client, in *ecv1beta1.Installation, ext ecv1beta1.Chart) error {
	return handleExtension(ctx, kcli, in, ext, actionUninstall, func() error {
		exists, err := hcli.ReleaseExists(ctx, ext.TargetNS, ext.Name)
		if err != nil {
			return errors.Wrap(err, "check if release exists")
		}
		if !exists {
			slog.Info("Extension already uninstalled", "name", ext.Name)
			return nil
		}
		if err := uninstall(ctx, hcli, ext); err != nil {
			return errors.Wrap(err, "uninstall")
		}
		return nil
	})
}

func handleExtension(ctx context.Context, kcli client.Client, in *ecv1beta1.Installation, ext ecv1beta1.Chart, action helmAction, processFn func() error) error {
	slogArgs := slogArgs(ext, action)

	if extensionAlreadyProcessed(in, ext) {
		slog.Info("Extension already processed", slogArgs...)
		return nil
	}

	slog.Info("Extension processing", slogArgs...)

	if action != actionNoChange {
		err := markExtensionAsProcessing(ctx, kcli, in, ext, action)
		if err != nil {
			return errors.Wrap(err, "mark extension as processing")
		}
	}

	err := processFn()
	if err != nil {
		if err := markExtensionAsFailed(ctx, kcli, in, ext, action, err); err != nil {
			slog.Error("Failed to mark extension as failed", append(slogArgs, "error", err)...)
		}
		return errors.Wrap(err, "process extension")
	}

	err = markExtensionAsProcessed(ctx, kcli, in, ext, action)
	if err != nil {
		return errors.Wrap(err, "mark extension as processed")
	}

	slog.Info("Extension is ready", slogArgs...)

	return nil
}

func extensionAlreadyProcessed(in *ecv1beta1.Installation, ext ecv1beta1.Chart) bool {
	conditionStatus := kubeutils.CheckInstallationConditionStatus(in.Status, conditionName(ext))
	return conditionStatus == metav1.ConditionTrue
}

func markExtensionAsProcessing(ctx context.Context, kcli client.Client, in *ecv1beta1.Installation, ext ecv1beta1.Chart, action helmAction) error {
	actionIng, _ := formatAction(action)
	if err := setCondition(ctx, kcli, in, conditionName(ext), metav1.ConditionFalse, actionIng, ""); err != nil {
		return errors.Wrap(err, "failed to set condition status")
	}
	return nil
}

func markExtensionAsProcessed(ctx context.Context, kcli client.Client, in *ecv1beta1.Installation, ext ecv1beta1.Chart, action helmAction) error {
	_, actionEd := formatAction(action)
	if err := setCondition(ctx, kcli, in, conditionName(ext), metav1.ConditionTrue, actionEd, ""); err != nil {
		return errors.Wrap(err, "failed to set condition status")
	}
	return nil
}

func markExtensionAsFailed(ctx context.Context, kcli client.Client, in *ecv1beta1.Installation, ext ecv1beta1.Chart, action helmAction, finalErr error) error {
	reason := string(action) + "Failed"
	message := helpers.CleanErrorMessage(finalErr)
	if err := setCondition(ctx, kcli, in, conditionName(ext), metav1.ConditionFalse, reason, message); err != nil {
		return errors.Wrap(err, "failed to set condition status")
	}
	return nil
}

func formatAction(action helmAction) (ing, ed string) {
	switch action {
	case actionInstall, actionUninstall:
		return string(action) + "ing", string(action) + "ed"
	case actionUpgrade:
		return "Upgrading", "Upgraded"
	default:
		return "Processing", "Processed"
	}
}

func setCondition(ctx context.Context, kcli client.Client, in *ecv1beta1.Installation, conditionType string, status metav1.ConditionStatus, reason, message string) error {
	return kubeutils.SetInstallationConditionStatus(ctx, kcli, in, metav1.Condition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	})
}

func slogArgs(ext ecv1beta1.Chart, action helmAction) []any {
	return []any{"name", ext.Name, "action", action}
}
