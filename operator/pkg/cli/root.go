package cli

import (
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/operator/controllers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

func RootCmd() *cobra.Command {
	var logLevel string
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string

	cmd := &cobra.Command{
		Use:          "manager",
		Short:        "Embedded Cluster Operator",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			lvl, err := logrus.ParseLevel(logLevel)
			if err != nil {
				return fmt.Errorf("failed to parse log level: %w", err)
			}
			err = setupCLILog(cmd, lvl)
			if err != nil {
				return fmt.Errorf("failed to setup log: %w", err)
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
				Scheme: kubeutils.Scheme,
				Metrics: metricsserver.Options{
					BindAddress: metricsAddr,
				},
				WebhookServer:                 webhook.NewServer(webhook.Options{Port: 9443}),
				HealthProbeBindAddress:        probeAddr,
				LeaderElection:                enableLeaderElection,
				LeaderElectionID:              "3f2343ef.replicated.com",
				LeaderElectionReleaseOnCancel: true,
			})
			if err != nil {
				setupLog.Error(err, "unable to start manager")
				os.Exit(1)
			}

			if err = (&controllers.InstallationReconciler{
				Client:    mgr.GetClient(),
				Scheme:    mgr.GetScheme(),
				Discovery: discovery.NewDiscoveryClientForConfigOrDie(ctrl.GetConfigOrDie()),
				Recorder:  mgr.GetEventRecorderFor("installation-controller"),
			}).SetupWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "Installation")
				os.Exit(1)
			}

			if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
				setupLog.Error(err, "unable to set up health check")
				os.Exit(1)
			}
			if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
				setupLog.Error(err, "unable to set up ready check")
				os.Exit(1)
			}

			setupLog.Info("Starting manager", "version", versions.Version, "k0sversion", versions.K0sVersion)
			if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
				setupLog.Error(err, "problem running manager")
				os.Exit(1)
			}
		},
	}

	addSubcommands(cmd)

	cmd.PersistentFlags().StringVar(&logLevel, "log-level", logrus.InfoLevel.String(), "Log level (debug, info, warn, error, fatal, panic)")

	cmd.Flags().StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	cmd.Flags().StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	cmd.Flags().BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	return cmd
}

func setupCLILog(cmd *cobra.Command, level logrus.Level) error {
	log, err := NewLogger(level)
	if err != nil {
		return err
	}
	ctx := ctrl.LoggerInto(cmd.Context(), log)
	cmd.SetContext(ctx)

	zaplog := zap.New(zap.UseDevMode(true))
	ctrl.SetLogger(zaplog)

	return nil
}

func addSubcommands(cmd *cobra.Command) {
	cmd.AddCommand(
		UpgradeCmd(),
		UpgradeJobCmd(),
		MigrateCmd(),
		MigrateV2Cmd(),
		VersionCmd(),
	)
}
