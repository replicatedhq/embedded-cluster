package linux

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/controllers/linux/install"
	"github.com/replicatedhq/embedded-cluster/api/controllers/linux/upgrade"
	linuxinstall "github.com/replicatedhq/embedded-cluster/api/internal/handlers/linux/install"
	linuxupgrade "github.com/replicatedhq/embedded-cluster/api/internal/handlers/linux/upgrade"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Handler struct {
	Install *linuxinstall.Handler
	Upgrade *linuxupgrade.Handler

	cfg               types.APIConfig
	installController install.Controller
	upgradeController upgrade.Controller
	logger            logrus.FieldLogger
	hostUtils         hostutils.HostUtilsInterface
	metricsReporter   metrics.ReporterInterface
	hcli              helm.Client
	kcli              client.Client
	mcli              metadata.Interface
	preflightRunner   preflights.PreflightRunnerInterface
}

type Option func(*Handler)

func WithInstallController(controller install.Controller) Option {
	return func(h *Handler) {
		h.installController = controller
	}
}

func WithUpgradeController(controller upgrade.Controller) Option {
	return func(h *Handler) {
		h.upgradeController = controller
	}
}

func WithLogger(logger logrus.FieldLogger) Option {
	return func(h *Handler) {
		h.logger = logger
	}
}

func WithHostUtils(hostUtils hostutils.HostUtilsInterface) Option {
	return func(h *Handler) {
		h.hostUtils = hostUtils
	}
}

func WithMetricsReporter(metricsReporter metrics.ReporterInterface) Option {
	return func(h *Handler) {
		h.metricsReporter = metricsReporter
	}
}

func WithHelmClient(hcli helm.Client) Option {
	return func(h *Handler) {
		h.hcli = hcli
	}
}

func WithKubeClient(kcli client.Client) Option {
	return func(h *Handler) {
		h.kcli = kcli
	}
}

func WithMetadataClient(mcli metadata.Interface) Option {
	return func(h *Handler) {
		h.mcli = mcli
	}
}

func WithPreflightRunner(preflightRunner preflights.PreflightRunnerInterface) Option {
	return func(h *Handler) {
		h.preflightRunner = preflightRunner
	}
}

func New(cfg types.APIConfig, opts ...Option) (*Handler, error) {
	h := &Handler{
		cfg: cfg,
	}

	for _, opt := range opts {
		opt(h)
	}

	if h.logger == nil {
		h.logger = logger.NewDiscardLogger()
	}

	if h.hostUtils == nil {
		h.hostUtils = hostutils.New(
			hostutils.WithLogger(h.logger),
		)
	}

	if h.cfg.Mode == types.ModeInstall {
		// TODO (@team): discuss which of these should / should not be pointers
		if h.installController == nil {
			installController, err := install.NewInstallController(
				install.WithRuntimeConfig(h.cfg.RuntimeConfig),
				install.WithLogger(h.logger),
				install.WithHostUtils(h.hostUtils),
				install.WithMetricsReporter(h.metricsReporter),
				install.WithReleaseData(h.cfg.ReleaseData),
				install.WithPassword(h.cfg.Password),
				install.WithTLSConfig(h.cfg.TLSConfig),
				install.WithLicense(h.cfg.License),
				install.WithAirgapBundle(h.cfg.AirgapBundle),
				install.WithAirgapMetadata(h.cfg.AirgapMetadata),
				install.WithEmbeddedAssetsSize(h.cfg.EmbeddedAssetsSize),
				install.WithConfigValues(h.cfg.ConfigValues),
				install.WithEndUserConfig(h.cfg.EndUserConfig),
				install.WithClusterID(h.cfg.ClusterID),
				install.WithAllowIgnoreHostPreflights(h.cfg.AllowIgnoreHostPreflights),
				install.WithHelmClient(h.hcli),
				install.WithKubeClient(h.kcli),
				install.WithMetadataClient(h.mcli),
				install.WithPreflightRunner(h.preflightRunner),
			)
			if err != nil {
				return nil, fmt.Errorf("new install controller: %w", err)
			}
			h.installController = installController
		}

		// Initialize sub-handler
		h.Install = linuxinstall.New(
			h.cfg,
			linuxinstall.WithController(h.installController),
			linuxinstall.WithLogger(h.logger),
			linuxinstall.WithHostUtils(h.hostUtils),
			linuxinstall.WithMetricsReporter(h.metricsReporter),
		)
	} else {
		// Initialize upgrade controller
		if h.upgradeController == nil {
			upgradeController, err := upgrade.NewUpgradeController(
				upgrade.WithRuntimeConfig(h.cfg.RuntimeConfig),
				upgrade.WithLogger(h.logger),
				upgrade.WithHostUtils(h.hostUtils),
				upgrade.WithMetricsReporter(h.metricsReporter),
				upgrade.WithReleaseData(h.cfg.ReleaseData),
				upgrade.WithLicense(h.cfg.License),
				upgrade.WithAirgapBundle(h.cfg.AirgapBundle),
				upgrade.WithAirgapMetadata(h.cfg.AirgapMetadata),
				upgrade.WithEmbeddedAssetsSize(h.cfg.EmbeddedAssetsSize),
				upgrade.WithConfigValues(h.cfg.ConfigValues),
				upgrade.WithEndUserConfig(h.cfg.EndUserConfig),
				upgrade.WithClusterID(h.cfg.ClusterID),
				upgrade.WithTargetVersion(h.cfg.TargetVersion),
				upgrade.WithInitialVersion(h.cfg.InitialVersion),
				upgrade.WithInfraUpgradeRequired(h.cfg.RequiresInfraUpgrade),
				upgrade.WithHelmClient(h.hcli),
				upgrade.WithKubeClient(h.kcli),
				upgrade.WithMetadataClient(h.mcli),
				upgrade.WithPreflightRunner(h.preflightRunner),
			)
			if err != nil {
				return nil, fmt.Errorf("new upgrade controller: %w", err)
			}
			h.upgradeController = upgradeController
		}

		// Initialize sub-handler
		h.Upgrade = linuxupgrade.New(
			h.cfg,
			linuxupgrade.WithController(h.upgradeController),
			linuxupgrade.WithLogger(h.logger),
		)
	}

	return h, nil
}
