package install

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/internal/managers/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/preflight"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/pkg/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
)

type Controller interface {
	GetInstallationConfig(ctx context.Context) (*types.InstallationConfig, error)
	ConfigureInstallation(ctx context.Context, config *types.InstallationConfig) error
	GetInstallationStatus(ctx context.Context) (*types.Status, error)
	RunHostPreflights(ctx context.Context) error
	GetHostPreflightStatus(ctx context.Context) (*types.Status, error)
	GetHostPreflightOutput(ctx context.Context) (*types.HostPreflightOutput, error)
	GetHostPreflightTitles(ctx context.Context) ([]string, error)
	SetStatus(ctx context.Context, status *types.Status) error
	ReadStatus(ctx context.Context) (*types.Status, error)
}

var _ Controller = (*InstallController)(nil)

type InstallController struct {
	install              *types.Install
	installationManager  installation.InstallationManager
	hostPreflightManager preflight.HostPreflightManager
	logger               logrus.FieldLogger
	hostUtils            *hostutils.HostUtils
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

func WithHostUtils(hostUtils *hostutils.HostUtils) InstallControllerOption {
	return func(c *InstallController) {
		c.hostUtils = hostUtils
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
	controller := &InstallController{
		install: types.NewInstall(),
	}

	for _, opt := range opts {
		opt(controller)
	}

	if controller.logger == nil {
		controller.logger = logger.NewDiscardLogger()
	}

	if controller.hostUtils == nil {
		controller.hostUtils = hostutils.New(
			hostutils.WithLogger(controller.logger),
		)
	}

	if controller.installationManager == nil {
		controller.installationManager = installation.NewInstallationManager(
			installation.WithLogger(controller.logger),
			installation.WithInstallation(controller.install.Steps.Installation),
		)
	}

	if controller.hostPreflightManager == nil {
		controller.hostPreflightManager = preflight.NewHostPreflightManager(
			preflight.WithLogger(controller.logger),
			preflight.WithMetricsReporter(controller.metricsReporter),
			preflight.WithHostPreflight(controller.install.Steps.HostPreflight),
		)
	}

	return controller, nil
}

func (c *InstallController) GetInstallationConfig(ctx context.Context) (*types.InstallationConfig, error) {
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

	return config, nil
}

func (c *InstallController) ConfigureInstallation(ctx context.Context, config *types.InstallationConfig) error {
	if err := c.installationManager.ValidateConfig(config); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	if err := c.computeCIDRs(config); err != nil {
		return fmt.Errorf("compute cidrs: %w", err)
	}

	if err := c.installationManager.WriteConfig(*config); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	go func() {
		// TODO NOW: other fields
		if err := c.hostUtils.ConfigureForInstall(ctx, hostutils.InitForInstallOptions{
			PodCIDR:     config.PodCIDR,
			ServiceCIDR: config.ServiceCIDR,
		}); err != nil {
			// TODO NOW: configure status like preflight status for ui to poll?
			c.logger.Errorf("configure for install: %v", err)
		}
	}()

	return nil
}

func (c *InstallController) GetInstallationStatus(ctx context.Context) (*types.Status, error) {
	return c.installationManager.ReadStatus()
}

func (c *InstallController) SetStatus(ctx context.Context, status *types.Status) error {
	if err := c.installationManager.WriteStatus(*status); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

func (c *InstallController) ReadStatus(ctx context.Context) (*types.Status, error) {
	return c.installationManager.ReadStatus()
}

func (c *InstallController) RunHostPreflights(ctx context.Context) error {
	// Get current installation config and add it to options
	config, err := c.installationManager.ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to read installation config: %w", err)
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
		return fmt.Errorf("failed to prepare host preflights: %w", err)
	}

	// Run host preflights
	return c.hostPreflightManager.RunHostPreflights(ctx, preflight.RunHostPreflightOptions{
		HostPreflightSpec: hpf,
		Proxy:             proxy,
		DataDirectory:     config.DataDirectory,
	})
}

func (c *InstallController) GetHostPreflightStatus(ctx context.Context) (*types.Status, error) {
	return c.hostPreflightManager.GetHostPreflightStatus(ctx)
}

func (c *InstallController) GetHostPreflightOutput(ctx context.Context) (*types.HostPreflightOutput, error) {
	return c.hostPreflightManager.GetHostPreflightOutput(ctx)
}

func (c *InstallController) GetHostPreflightTitles(ctx context.Context) ([]string, error) {
	return c.hostPreflightManager.GetHostPreflightTitles(ctx)
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
