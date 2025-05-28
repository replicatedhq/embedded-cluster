package preflight

import (
	"context"
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/sirupsen/logrus"
)

// HostPreflightManager provides methods for running host preflights
type HostPreflightManager interface {
	PrepareHostPreflights(ctx context.Context, opts PrepareHostPreflightOptions) (*troubleshootv1beta2.HostPreflightSpec, *ecv1beta1.ProxySpec, error)
	RunHostPreflights(ctx context.Context, options RunHostPreflightOptions) (*types.RunHostPreflightResponse, error)
	GetHostPreflightStatus(ctx context.Context) (*types.HostPreflightStatusResponse, error)
}

type hostPreflightManager struct {
	logger logrus.FieldLogger

	// Thread-safe execution state
	mu        sync.RWMutex
	status    types.HostPreflightStatus
	output    *types.HostPreflightOutput
	isRunning bool
}

type HostPreflightManagerOption func(*hostPreflightManager)

func WithLogger(logger logrus.FieldLogger) HostPreflightManagerOption {
	return func(m *hostPreflightManager) {
		m.logger = logger
	}
}

// NewHostPreflightManager creates a new HostPreflightManager
func NewHostPreflightManager(opts ...HostPreflightManagerOption) HostPreflightManager {
	manager := &hostPreflightManager{}

	for _, opt := range opts {
		opt(manager)
	}

	if manager.logger == nil {
		manager.logger = logger.NewDiscardLogger()
	}

	return manager
}
