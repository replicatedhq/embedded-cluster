package release

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/pkg/template"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/sirupsen/logrus"
)

// AppReleaseManager provides methods for managing the release of an app
type AppReleaseManager interface {
	ExtractAppPreflightSpec(ctx context.Context, configValues types.AppConfigValues, proxySpec *ecv1beta1.ProxySpec, registrySettings *types.RegistrySettings) (*troubleshootv1beta2.PreflightSpec, error)
	ExtractInstallableHelmCharts(ctx context.Context, configValues types.AppConfigValues, proxySpec *ecv1beta1.ProxySpec, registrySettings *types.RegistrySettings) ([]types.InstallableHelmChart, error)
}

type appReleaseManager struct {
	rawConfig                  kotsv1beta1.Config
	releaseData                *release.ReleaseData
	templateEngine             *template.Engine
	license                    *kotsv1beta1.License
	logger                     logrus.FieldLogger
	privateCACertConfigMapName string
	hcli                       helm.Client
}

type AppReleaseManagerOption func(*appReleaseManager)

func WithLogger(logger logrus.FieldLogger) AppReleaseManagerOption {
	return func(m *appReleaseManager) {
		m.logger = logger
	}
}

func WithReleaseData(releaseData *release.ReleaseData) AppReleaseManagerOption {
	return func(m *appReleaseManager) {
		m.releaseData = releaseData
	}
}

func WithTemplateEngine(templateEngine *template.Engine) AppReleaseManagerOption {
	return func(m *appReleaseManager) {
		m.templateEngine = templateEngine
	}
}

func WithLicense(license *kotsv1beta1.License) AppReleaseManagerOption {
	return func(m *appReleaseManager) {
		m.license = license
	}
}

func WithPrivateCACertConfigMapName(configMapName string) AppReleaseManagerOption {
	return func(m *appReleaseManager) {
		m.privateCACertConfigMapName = configMapName
	}
}

func WithHelmClient(hcli helm.Client) AppReleaseManagerOption {
	return func(m *appReleaseManager) {
		m.hcli = hcli
	}
}

// NewAppReleaseManager creates a new AppReleaseManager
func NewAppReleaseManager(config kotsv1beta1.Config, opts ...AppReleaseManagerOption) (AppReleaseManager, error) {
	manager := &appReleaseManager{
		rawConfig: config,
	}

	for _, opt := range opts {
		opt(manager)
	}

	if manager.releaseData == nil {
		return nil, fmt.Errorf("release data not found")
	}

	if manager.hcli == nil {
		return nil, fmt.Errorf("helm client is required")
	}

	if manager.logger == nil {
		manager.logger = logger.NewDiscardLogger()
	}

	if manager.templateEngine == nil {
		manager.templateEngine = template.NewEngine(
			&manager.rawConfig,
			template.WithLicense(manager.license),
			template.WithReleaseData(manager.releaseData),
			template.WithPrivateCACertConfigMapName(manager.privateCACertConfigMapName),
		)
	}

	return manager, nil
}
