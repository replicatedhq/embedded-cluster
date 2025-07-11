package linux

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/controllers/linux/install"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	cfg               types.APIConfig
	installController install.Controller
	logger            logrus.FieldLogger
	hostUtils         hostutils.HostUtilsInterface
	metricsReporter   metrics.ReporterInterface
}

type Option func(*Handler)

func WithInstallController(controller install.Controller) Option {
	return func(h *Handler) {
		h.installController = controller
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
			install.WithConfigValues(h.cfg.ConfigValues),
			install.WithEndUserConfig(h.cfg.EndUserConfig),
			install.WithClusterID(h.cfg.ClusterID),
			install.WithAllowIgnoreHostPreflights(h.cfg.AllowIgnoreHostPreflights),
		)
		if err != nil {
			return nil, fmt.Errorf("new install controller: %w", err)
		}
		h.installController = installController
	}

	return h, nil
}
