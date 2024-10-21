package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/replicatedhq/embedded-cluster/operator/pkg/k8sutil"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/upgrade"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/spf13/cobra"
)

// UpgradeJobCmd returns a cobra command for upgrading the embedded cluster operator.
// It is called by KOTS admin console to upgrade the embedded cluster operator and installation.
func UpgradeJobCmd() *cobra.Command {
	var installationFile, previousInstallationVersion string

	cmd := &cobra.Command{
		Use:          "upgrade-job",
		Short:        "Upgrade k0s and then all addons from within a job that may be restarted",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Upgrade job version %s started\n", versions.Version)

			cli, err := k8sutil.KubeClient()
			if err != nil {
				return fmt.Errorf("failed to create kubernetes client: %w", err)
			}

			installationData, err := readInstallationFile(installationFile)
			if err != nil {
				return fmt.Errorf("failed to read installation file: %w", err)
			}

			fmt.Printf("installation data: %s\n", installationData)

			in, err := decodeInstallation(cmd.Context(), []byte(installationData))
			if err != nil {
				return fmt.Errorf("failed to decode installation: %w", err)
			}

			fmt.Printf("Upgrading to installation %s (version %s)\n", in.Name, in.Spec.Config.Version)

			i := 0
			allErrors := []string{}
			sleepDuration := time.Second * 5
			for {
				err = upgrade.Upgrade(cmd.Context(), cli, in)
				if err != nil {
					fmt.Printf("Upgrade failed, retrying: %s\n", err.Error())
					allErrors = append(allErrors, err.Error())
					if i >= 10 {
						if !in.Spec.AirGap {
							err = metrics.NotifyUpgradeFailed(cmd.Context(), in.Spec.MetricsBaseURL, metrics.UpgradeFailedEvent{
								ClusterID:      in.Spec.ClusterID,
								TargetVersion:  in.Spec.Config.Version,
								InitialVersion: previousInstallationVersion,
								Reason:         strings.Join(allErrors, ", "),
							})
							if err != nil {
								fmt.Printf("failed to report that the upgrade was started: %v\n", err)
							}
						}
						return fmt.Errorf("failed to upgrade after %s", (sleepDuration * time.Duration(i)).String())
					}

					time.Sleep(sleepDuration)
					i++
					continue
				}
				break
			}

			fmt.Println("Upgrade completed successfully")
			return nil
		},
	}

	cmd.Flags().StringVar(&installationFile, "installation", "", "Path to the installation file")
	cmd.Flags().StringVar(&previousInstallationVersion, "previous-version", "", "the previous installation version")
	err := cmd.MarkFlagRequired("installation")
	if err != nil {
		panic(err)
	}
	err = cmd.MarkFlagRequired("previous-version")
	if err != nil {
		panic(err)
	}

	return cmd
}
