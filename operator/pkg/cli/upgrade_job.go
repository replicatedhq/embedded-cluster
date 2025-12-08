package cli

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/cli/migratev2"
	"github.com/replicatedhq/embedded-cluster/pkg-new/upgrade"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// UpgradeJobCmd returns a cobra command for upgrading the embedded cluster operator.
// It is called by KOTS admin console to upgrade the embedded cluster operator and installation.
func UpgradeJobCmd() *cobra.Command {
	var inFile, previousInVersion string
	var in *ecv1beta1.Installation

	rc := runtimeconfig.New(nil)

	cmd := &cobra.Command{
		Use:          "upgrade-job",
		Short:        "Upgrade k0s and then all addons from within a job that may be restarted",
		SilenceUsage: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			var err error
			in, err = getInstallationFromFile(inFile)
			if err != nil {
				return fmt.Errorf("failed to get installation from file: %w", err)
			}

			// set the runtime config from the installation spec
			rc.Set(in.Spec.RuntimeConfig)

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logrus.New()
			logger.WithField("version", versions.Version).Info("Upgrade job started")
			logger.WithFields(logrus.Fields{
				"name":    in.Name,
				"version": in.Spec.Config.Version,
			}).Info("Upgrading to installation")

			kcli, err := kubeutils.KubeClient()
			if err != nil {
				return fmt.Errorf("failed to create kubernetes client: %w", err)
			}

			airgapChartsPath := ""
			if in.Spec.AirGap {
				airgapChartsPath = rc.EmbeddedClusterChartsSubDirNoCreate()
			}

			hcli, err := helm.NewClient(helm.HelmOptions{
				HelmPath:   "helm", // use the helm binary in PATH
				K8sVersion: versions.K0sVersion,
				AirgapPath: airgapChartsPath,
			})
			if err != nil {
				return fmt.Errorf("failed to create helm client: %w", err)
			}
			defer hcli.Close()

			if upgradeErr := performUpgrade(cmd.Context(), kcli, hcli, rc, in, logger); upgradeErr != nil {
				// if this is the last attempt, mark the installation as failed
				if err := maybeMarkAsFailed(cmd.Context(), kcli, in, upgradeErr); err != nil {
					logger.WithError(err).Error("Failed to mark installation as failed")
				}
				return upgradeErr
			}

			logger.Info("Upgrade completed successfully")

			return nil
		},
	}

	cmd.Flags().StringVar(&inFile, "installation", "", "Path to the installation file")
	err := cmd.MarkFlagRequired("installation")
	if err != nil {
		panic(err)
	}
	cmd.Flags().StringVar(&previousInVersion, "previous-version", "", "the previous installation version")
	err = cmd.MarkFlagRequired("previous-version")
	if err != nil {
		panic(err)
	}

	return cmd
}

func performUpgrade(ctx context.Context, kcli client.Client, hcli helm.Client, rc runtimeconfig.RuntimeConfig, in *ecv1beta1.Installation, logger logrus.FieldLogger) (finalErr error) {
	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("upgrade recovered from panic: %v: %s", r, string(debug.Stack()))
		}
	}()

	if err := migratev2.Run(ctx, kcli, in, logger); err != nil {
		return fmt.Errorf("failed to run v2 migration: %w", err)
	}

	if err := upgrade.Upgrade(ctx, kcli, hcli, rc, in, logger); err != nil {
		return err
	}
	return nil
}

func maybeMarkAsFailed(ctx context.Context, kcli client.Client, in *ecv1beta1.Installation, upgradeErr error) error {
	lastAttempt, err := isLastAttempt(ctx, kcli)
	if err != nil {
		return fmt.Errorf("check if last attempt: %w", err)
	}
	if !lastAttempt {
		return nil
	}
	if err := kubeutils.SetInstallationState(ctx, kcli, in, ecv1beta1.InstallationStateFailed, helpers.CleanErrorMessage(upgradeErr)); err != nil {
		return fmt.Errorf("set installation state: %w", err)
	}
	return nil
}

func isLastAttempt(ctx context.Context, kcli client.Client) (bool, error) {
	var job batchv1.Job
	nsn := types.NamespacedName{Name: os.Getenv("JOB_NAME"), Namespace: os.Getenv("JOB_NAMESPACE")}
	if err := kcli.Get(ctx, nsn, &job); err != nil {
		return false, fmt.Errorf("get upgrade job: %w", err)
	}

	if job.Spec.BackoffLimit == nil {
		return false, fmt.Errorf("job backoff limit is nil")
	}

	return job.Status.Failed >= *job.Spec.BackoffLimit, nil
}
