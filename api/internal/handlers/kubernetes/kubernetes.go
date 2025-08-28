package kubernetes

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/controllers/kubernetes/install"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/sirupsen/logrus"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type Handler struct {
	cfg               types.APIConfig
	installController install.Controller
	logger            logrus.FieldLogger
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
		var restClientGetter genericclioptions.RESTClientGetter
		if ks := h.cfg.Installation.GetKubernetesEnvSettings(); ks != nil {
			restClientGetter = ks.RESTClientGetter()
		}
		k8sVersion, err := getK8sVersion(restClientGetter)
		if err != nil {
			return nil, fmt.Errorf("get k8s version: %w", err)
		}

		installController, err := install.NewInstallController(
			install.WithLogger(h.logger),
			install.WithMetricsReporter(h.metricsReporter),
			install.WithK8sVersion(k8sVersion),
			install.WithKubernetesEnvSettings(h.cfg.Installation.GetKubernetesEnvSettings()),
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

	return h, nil
}
