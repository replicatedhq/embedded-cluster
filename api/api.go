package api

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/controllers/auth"
	"github.com/replicatedhq/embedded-cluster/api/controllers/console"
	kubernetesinstall "github.com/replicatedhq/embedded-cluster/api/controllers/kubernetes/install"
	kubernetesupgrade "github.com/replicatedhq/embedded-cluster/api/controllers/kubernetes/upgrade"
	kurlmigration "github.com/replicatedhq/embedded-cluster/api/controllers/kurlmigration"
	linuxinstall "github.com/replicatedhq/embedded-cluster/api/controllers/linux/install"
	linuxupgrade "github.com/replicatedhq/embedded-cluster/api/controllers/linux/upgrade"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// API represents the main HTTP API server for the Embedded Cluster application.
//
//	@title			Embedded Cluster API
//	@version		0.1
//	@description	This is the API for the Embedded Cluster project.
//	@termsOfService	http://swagger.io/terms/

//	@contact.name	API Support
//	@contact.url	https://github.com/replicatedhq/embedded-cluster/issues
//	@contact.email	support@replicated.com

//	@license.name	Apache 2.0
//	@license.url	http://www.apache.org/licenses/LICENSE-2.0.html

//	@BasePath	/api

//	@securityDefinitions.bearerauth	bearerauth

// @externalDocs.description	OpenAPI
// @externalDocs.url			https://swagger.io/resources/open-api/
type API struct {
	cfg types.APIConfig

	hcli            helm.Client
	kcli            client.Client
	mcli            metadata.Interface
	preflightRunner preflights.PreflightRunnerInterface
	logger          logrus.FieldLogger
	metricsReporter metrics.ReporterInterface

	authController              auth.Controller
	consoleController           console.Controller
	linuxInstallController      linuxinstall.Controller
	linuxUpgradeController      linuxupgrade.Controller
	kubernetesInstallController kubernetesinstall.Controller
	kubernetesUpgradeController kubernetesupgrade.Controller
	kurlMigrationController     kurlmigration.Controller

	handlers handlers
}

// Option is a function that configures the API.
type Option func(*API)

// WithAuthController configures the auth controller for the API.
func WithAuthController(authController auth.Controller) Option {
	return func(a *API) {
		a.authController = authController
	}
}

// WithConsoleController configures the console controller for the API.
func WithConsoleController(consoleController console.Controller) Option {
	return func(a *API) {
		a.consoleController = consoleController
	}
}

// WithLinuxInstallController configures the linux install controller for the API.
func WithLinuxInstallController(linuxInstallController linuxinstall.Controller) Option {
	return func(a *API) {
		a.linuxInstallController = linuxInstallController
	}
}

// WithLinuxUpgradeController configures the linux upgrade controller for the API.
func WithLinuxUpgradeController(linuxUpgradeController linuxupgrade.Controller) Option {
	return func(a *API) {
		a.linuxUpgradeController = linuxUpgradeController
	}
}

// WithKubernetesInstallController configures the kubernetes install controller for the API.
func WithKubernetesInstallController(kubernetesInstallController kubernetesinstall.Controller) Option {
	return func(a *API) {
		a.kubernetesInstallController = kubernetesInstallController
	}
}

// WithKubernetesUpgradeController configures the kubernetes upgrade controller for the API.
func WithKubernetesUpgradeController(kubernetesUpgradeController kubernetesupgrade.Controller) Option {
	return func(a *API) {
		a.kubernetesUpgradeController = kubernetesUpgradeController
	}
}

// WithKURLMigrationController configures the kURL migration controller for the API.
func WithKURLMigrationController(kurlMigrationController kurlmigration.Controller) Option {
	return func(a *API) {
		a.kurlMigrationController = kurlMigrationController
	}
}

// WithLogger configures the logger for the API. If not provided, a default logger will be created.
func WithLogger(logger logrus.FieldLogger) Option {
	return func(a *API) {
		a.logger = logger
	}
}

// WithMetricsReporter configures the metrics reporter for the API.
func WithMetricsReporter(metricsReporter metrics.ReporterInterface) Option {
	return func(a *API) {
		a.metricsReporter = metricsReporter
	}
}

// WithHelmClient configures the helm client for the API.
func WithHelmClient(hcli helm.Client) Option {
	return func(a *API) {
		a.hcli = hcli
	}
}

// WithKubeClient configures the kube client for the API.
func WithKubeClient(kcli client.Client) Option {
	return func(a *API) {
		a.kcli = kcli
	}
}

// WithMetadataClient configures the metadata client for the API.
func WithMetadataClient(mcli metadata.Interface) Option {
	return func(a *API) {
		a.mcli = mcli
	}
}

// WithPreflightRunner configures the preflight runner for the API.
func WithPreflightRunner(preflightRunner preflights.PreflightRunnerInterface) Option {
	return func(a *API) {
		a.preflightRunner = preflightRunner
	}
}

// New creates a new API instance.
func New(cfg types.APIConfig, opts ...Option) (*API, error) {
	if cfg.InstallTarget == "" {
		return nil, fmt.Errorf("target is required")
	}

	api := &API{
		cfg: cfg,
	}

	for _, opt := range opts {
		opt(api)
	}

	if api.cfg.RuntimeConfig == nil {
		api.cfg.RuntimeConfig = runtimeconfig.New(nil)
	}

	if api.logger == nil {
		l, err := logger.NewLogger()
		if err != nil {
			return nil, fmt.Errorf("create logger: %w", err)
		}
		api.logger = l
	}

	// Create kURL migration controller with file-based persistence if not provided
	if api.kurlMigrationController == nil && cfg.InstallTarget == types.InstallTargetLinux {
		dataDir := api.cfg.RuntimeConfig.EmbeddedClusterHomeDirectory()
		store := store.NewStoreWithDataDir(dataDir)
		controller, err := kurlmigration.NewKURLMigrationController(
			kurlmigration.WithStore(store),
			kurlmigration.WithLogger(api.logger),
		)
		if err != nil {
			return nil, fmt.Errorf("create kurl migration controller: %w", err)
		}
		api.kurlMigrationController = controller
	}

	if err := api.initClients(); err != nil {
		return nil, fmt.Errorf("init clients: %w", err)
	}

	if err := api.initHandlers(); err != nil {
		return nil, fmt.Errorf("init handlers: %w", err)
	}

	return api, nil
}
