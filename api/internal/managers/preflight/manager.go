package preflight

import (
	"context"
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/sirupsen/logrus"
)

// HostPreflightManager provides methods for running host preflights
type HostPreflightManager interface {
	PrepareHostPreflights(ctx context.Context, opts PrepareHostPreflightOptions) (*troubleshootv1beta2.HostPreflightSpec, *ecv1beta1.ProxySpec, error)
	RunHostPreflights(ctx context.Context, opts RunHostPreflightOptions) error
	GetHostPreflightStatus(ctx context.Context) (*types.Status, error)
	GetHostPreflightOutput(ctx context.Context) (*types.HostPreflightsOutput, error)
	GetHostPreflightTitles(ctx context.Context) ([]string, error)
}

type hostPreflightManager struct {
	hostPreflightStore HostPreflightStore
	rc                 runtimeconfig.RuntimeConfig
	logger             logrus.FieldLogger
	metricsReporter    metrics.ReporterInterface
	mu                 sync.RWMutex
}

type HostPreflightManagerOption func(*hostPreflightManager)

func WithRuntimeConfig(rc runtimeconfig.RuntimeConfig) HostPreflightManagerOption {
	return func(m *hostPreflightManager) {
		m.rc = rc
	}
}

func WithLogger(logger logrus.FieldLogger) HostPreflightManagerOption {
	return func(m *hostPreflightManager) {
		m.logger = logger
	}
}

func WithMetricsReporter(metricsReporter metrics.ReporterInterface) HostPreflightManagerOption {
	return func(m *hostPreflightManager) {
		m.metricsReporter = metricsReporter
	}
}

func WithHostPreflightStore(hostPreflightStore HostPreflightStore) HostPreflightManagerOption {
	return func(m *hostPreflightManager) {
		m.hostPreflightStore = hostPreflightStore
	}
}

// NewHostPreflightManager creates a new HostPreflightManager
func NewHostPreflightManager(opts ...HostPreflightManagerOption) HostPreflightManager {
	manager := &hostPreflightManager{}

	for _, opt := range opts {
		opt(manager)
	}

	if manager.rc == nil {
		manager.rc = runtimeconfig.New(nil)
	}

	if manager.logger == nil {
		manager.logger = logger.NewDiscardLogger()
	}

	if manager.hostPreflightStore == nil {
		manager.hostPreflightStore = NewMemoryStore(types.NewHostPreflights())
	}

	return manager
}
