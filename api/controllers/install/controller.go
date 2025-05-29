package install

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/internal/managers/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/preflight"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/pkg/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
)

type Controller interface {
	Get(ctx context.Context) (*types.Install, error)
	SetConfig(ctx context.Context, config *types.InstallationConfig) error
	SetStatus(ctx context.Context, status *types.InstallationStatus) error
	ReadStatus(ctx context.Context) (*types.InstallationStatus, error)
	RunHostPreflights(ctx context.Context) (*types.RunHostPreflightResponse, error)
	GetHostPreflightStatus(ctx context.Context) (*types.HostPreflightStatusResponse, error)
}

var _ Controller = (*InstallController)(nil)

type InstallController struct {
	installationManager  installation.InstallationManager
	hostPreflightManager preflight.HostPreflightManager
	logger               logrus.FieldLogger
	metricsReporter      metrics.ReporterInterface
	releaseData          *release.ReleaseData
	isAirgap             bool
}

type InstallControllerOption func(*InstallController)

func WithLogger(logger logrus.FieldLogger) InstallControllerOption {
	return func(c *InstallController) {
		c.logger = logger
	}
}

func WithMetricsReporter(metricsReporter metrics.ReporterInterface) InstallControllerOption {
	return func(c *InstallController) {
		c.metricsReporter = metricsReporter
	}
}

func WithReleaseData(releaseData *release.ReleaseData) InstallControllerOption {
	return func(c *InstallController) {
		c.releaseData = releaseData
	}
}

func WithIsAirgap(isAirgap bool) InstallControllerOption {
	return func(c *InstallController) {
		c.isAirgap = isAirgap
	}
}

func WithInstallationManager(installationManager installation.InstallationManager) InstallControllerOption {
	return func(c *InstallController) {
		c.installationManager = installationManager
	}
}

func WithHostPreflightManager(hostPreflightManager preflight.HostPreflightManager) InstallControllerOption {
	return func(c *InstallController) {
		c.hostPreflightManager = hostPreflightManager
	}
}

func NewInstallController(opts ...InstallControllerOption) (*InstallController, error) {
	controller := &InstallController{}

	for _, opt := range opts {
		opt(controller)
	}

	if controller.logger == nil {
		controller.logger = logger.NewDiscardLogger()
	}

	if controller.installationManager == nil {
		controller.installationManager = installation.NewInstallationManager(
			installation.WithLogger(controller.logger),
		)
	}

	if controller.hostPreflightManager == nil {
		controller.hostPreflightManager = preflight.NewHostPreflightManager(
			preflight.WithLogger(controller.logger),
			preflight.WithMetricsReporter(controller.metricsReporter),
		)
	}

	return controller, nil
}

func (c *InstallController) Get(ctx context.Context) (*types.Install, error) {
	config, err := c.installationManager.ReadConfig()
	if err != nil {
		return nil, err
	}

	if err := c.installationManager.SetConfigDefaults(config); err != nil {
		return nil, fmt.Errorf("set defaults: %w", err)
	}

	if err := c.installationManager.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	status, err := c.installationManager.ReadStatus()
	if err != nil {
		return nil, fmt.Errorf("read status: %w", err)
	}

	install := &types.Install{
		Config: *config,
		Status: *status,
	}

	return install, nil
}

func (c *InstallController) SetConfig(ctx context.Context, config *types.InstallationConfig) error {
	if err := c.installationManager.ValidateConfig(config); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	if err := c.computeCIDRs(config); err != nil {
		return fmt.Errorf("compute cidrs: %w", err)
	}

	if err := c.installationManager.WriteConfig(*config); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

func (c *InstallController) SetStatus(ctx context.Context, status *types.InstallationStatus) error {
	if err := c.installationManager.WriteStatus(*status); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

func (c *InstallController) ReadStatus(ctx context.Context) (*types.InstallationStatus, error) {
	return c.installationManager.ReadStatus()
}

func (c *InstallController) RunHostPreflights(ctx context.Context) (*types.RunHostPreflightResponse, error) {
	// Get current installation config and add it to options
	config, err := c.installationManager.ReadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to read installation config: %w", err)
	}

	// Get the configured custom domains
	ecDomains := utils.GetDomains(c.releaseData)

	// Prepare host preflights
	hpf, proxy, err := c.hostPreflightManager.PrepareHostPreflights(ctx, preflight.PrepareHostPreflightOptions{
		InstallationConfig:    config,
		ReplicatedAppURL:      netutils.MaybeAddHTTPS(ecDomains.ReplicatedAppDomain),
		ProxyRegistryURL:      netutils.MaybeAddHTTPS(ecDomains.ProxyRegistryDomain),
		HostPreflightSpec:     c.releaseData.HostPreflights,
		EmbeddedClusterConfig: c.releaseData.EmbeddedClusterConfig,
		IsAirgap:              c.isAirgap,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to prepare host preflights: %w", err)
	}

	// Run host preflights
	return c.hostPreflightManager.RunHostPreflights(ctx, preflight.RunHostPreflightOptions{
		HostPreflightSpec: hpf,
		Proxy:             proxy,
		DataDirectory:     config.DataDirectory,
	})
}

func (c *InstallController) GetHostPreflightStatus(ctx context.Context) (*types.HostPreflightStatusResponse, error) {
	return c.hostPreflightManager.GetHostPreflightStatus(ctx)
}

func (c *InstallController) computeCIDRs(config *types.InstallationConfig) error {
	if config.GlobalCIDR != "" {
		podCIDR, serviceCIDR, err := netutils.SplitNetworkCIDR(config.GlobalCIDR)
		if err != nil {
			return fmt.Errorf("split network cidr: %w", err)
		}
		config.PodCIDR = podCIDR
		config.ServiceCIDR = serviceCIDR
	}

	return nil
}
