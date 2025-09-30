package kubernetes

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/controllers/kubernetes/install"
	"github.com/replicatedhq/embedded-cluster/api/controllers/kubernetes/upgrade"
	k8sinstall "github.com/replicatedhq/embedded-cluster/api/internal/handlers/kubernetes/install"
	k8supgrade "github.com/replicatedhq/embedded-cluster/api/internal/handlers/kubernetes/upgrade"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	Install *k8sinstall.Handler
	Upgrade *k8supgrade.Handler

	cfg               types.APIConfig
	installController install.Controller
	upgradeController upgrade.Controller
	logger            logrus.FieldLogger
	metricsReporter   metrics.ReporterInterface
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

func WithMetricsReporter(metricsReporter metrics.ReporterInterface) Option {
	return func(h *Handler) {
		h.metricsReporter = metricsReporter
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

	// TODO (@team): discuss which of these should / should not be pointers
	if h.installController == nil {
		installController, err := install.NewInstallController(
			install.WithLogger(h.logger),
			install.WithMetricsReporter(h.metricsReporter),
			install.WithRESTClientGetter(h.cfg.RESTClientGetter),
			install.WithReleaseData(h.cfg.ReleaseData),
			install.WithConfigValues(h.cfg.ConfigValues),
			install.WithEndUserConfig(h.cfg.EndUserConfig),
			install.WithPassword(h.cfg.Password),
			//nolint:staticcheck // QF1008 this is very ambiguous, we should re-think the config struct
			install.WithInstallation(h.cfg.KubernetesConfig.Installation),
		)
		if err != nil {
			return nil, fmt.Errorf("new install controller: %w", err)
		}
		h.installController = installController
	}

	// Initialize upgrade controller if upgrade is supported
	if h.upgradeController == nil {
		upgradeController, err := upgrade.NewUpgradeController(
			upgrade.WithLogger(h.logger),
			upgrade.WithRESTClientGetter(h.cfg.RESTClientGetter),
			upgrade.WithReleaseData(h.cfg.ReleaseData),
			upgrade.WithLicense(h.cfg.License),
			upgrade.WithAirgapBundle(h.cfg.AirgapBundle),
			upgrade.WithConfigValues(h.cfg.ConfigValues),
		)
		if err != nil {
			return nil, fmt.Errorf("new upgrade controller: %w", err)
		}
		h.upgradeController = upgradeController
	}

	// Initialize sub-handlers
	h.Install = k8sinstall.New(
		h.cfg,
		k8sinstall.WithController(h.installController),
		k8sinstall.WithLogger(h.logger),
		k8sinstall.WithMetricsReporter(h.metricsReporter),
	)

	h.Upgrade = k8supgrade.New(
		h.cfg,
		k8supgrade.WithController(h.upgradeController),
		k8supgrade.WithLogger(h.logger),
	)

	return h, nil
}
