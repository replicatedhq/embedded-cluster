package install

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/client"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
)

// mockAPIClient is a mock implementation of the client.Client interface for testing
type mockAPIClient struct {
	authenticateFunc                        func(ctx context.Context, password string) error
	patchLinuxInstallAppConfigValuesFunc    func(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfigValues, error)
	getLinuxInstallationConfigFunc          func(ctx context.Context) (apitypes.LinuxInstallationConfigResponse, error)
	getLinuxInstallationStatusFunc          func(ctx context.Context) (apitypes.Status, error)
	configureLinuxInstallationFunc          func(ctx context.Context, config apitypes.LinuxInstallationConfig) (apitypes.Status, error)
	runLinuxInstallHostPreflightsFunc       func(ctx context.Context) (apitypes.InstallHostPreflightsStatusResponse, error)
	getLinuxInstallHostPreflightsStatusFunc func(ctx context.Context) (apitypes.InstallHostPreflightsStatusResponse, error)
	setupLinuxInfraFunc                     func(ctx context.Context, ignoreHostPreflights bool) (apitypes.Infra, error)
	getLinuxInfraStatusFunc                 func(ctx context.Context) (apitypes.Infra, error)
	processLinuxAirgapFunc                  func(ctx context.Context) (apitypes.Airgap, error)
	getLinuxAirgapStatusFunc                func(ctx context.Context) (apitypes.Airgap, error)
	getLinuxInstallAppConfigValuesFunc      func(ctx context.Context) (apitypes.AppConfigValues, error)
	templateLinuxInstallAppConfigFunc       func(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfig, error)
	runLinuxInstallAppPreflightsFunc        func(ctx context.Context) (apitypes.InstallAppPreflightsStatusResponse, error)
	getLinuxInstallAppPreflightsStatusFunc  func(ctx context.Context) (apitypes.InstallAppPreflightsStatusResponse, error)
	installLinuxAppFunc                     func(ctx context.Context, ignoreAppPreflights bool) (apitypes.AppInstall, error)
	getLinuxAppInstallStatusFunc            func(ctx context.Context) (apitypes.AppInstall, error)
	getLinuxUpgradeAppConfigValuesFunc      func(ctx context.Context) (apitypes.AppConfigValues, error)
	patchLinuxUpgradeAppConfigValuesFunc    func(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfigValues, error)
	templateLinuxUpgradeAppConfigFunc       func(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfig, error)
	runLinuxUpgradeAppPreflightsFunc        func(ctx context.Context) (apitypes.UpgradeAppPreflightsStatusResponse, error)
	getLinuxUpgradeAppPreflightsStatusFunc  func(ctx context.Context) (apitypes.UpgradeAppPreflightsStatusResponse, error)
	upgradeLinuxAppFunc                     func(ctx context.Context, ignoreAppPreflights bool) (apitypes.AppUpgrade, error)
	getLinuxAppUpgradeStatusFunc            func(ctx context.Context) (apitypes.AppUpgrade, error)
	upgradeLinuxInfraFunc                      func(ctx context.Context, ignoreHostPreflights bool) (apitypes.Infra, error)
	getLinuxUpgradeInfraStatusFunc             func(ctx context.Context) (apitypes.Infra, error)
	processLinuxUpgradeAirgapFunc              func(ctx context.Context) (apitypes.Airgap, error)
	getLinuxUpgradeAirgapStatusFunc            func(ctx context.Context) (apitypes.Airgap, error)
	runLinuxUpgradeHostPreflightsFunc          func(ctx context.Context) (apitypes.HostPreflights, error)
	getLinuxUpgradeHostPreflightsStatusFunc    func(ctx context.Context) (apitypes.HostPreflights, error)

	// Kubernetes methods (not used in current implementation, but required by interface)
	getKubernetesInstallationConfigFunc         func(ctx context.Context) (apitypes.KubernetesInstallationConfigResponse, error)
	configureKubernetesInstallationFunc         func(ctx context.Context, config apitypes.KubernetesInstallationConfig) (apitypes.Status, error)
	getKubernetesInstallationStatusFunc         func(ctx context.Context) (apitypes.Status, error)
	setupKubernetesInfraFunc                    func(ctx context.Context) (apitypes.Infra, error)
	getKubernetesInfraStatusFunc                func(ctx context.Context) (apitypes.Infra, error)
	getKubernetesInstallAppConfigValuesFunc     func(ctx context.Context) (apitypes.AppConfigValues, error)
	patchKubernetesInstallAppConfigValuesFunc   func(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfigValues, error)
	templateKubernetesInstallAppConfigFunc      func(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfig, error)
	runKubernetesInstallAppPreflightsFunc       func(ctx context.Context) (apitypes.InstallAppPreflightsStatusResponse, error)
	getKubernetesInstallAppPreflightsStatusFunc func(ctx context.Context) (apitypes.InstallAppPreflightsStatusResponse, error)
	installKubernetesAppFunc                    func(ctx context.Context) (apitypes.AppInstall, error)
	getKubernetesAppInstallStatusFunc           func(ctx context.Context) (apitypes.AppInstall, error)
	getKubernetesUpgradeAppConfigValuesFunc     func(ctx context.Context) (apitypes.AppConfigValues, error)
	patchKubernetesUpgradeAppConfigValuesFunc   func(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfigValues, error)
	templateKubernetesUpgradeAppConfigFunc      func(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfig, error)
}

func (m *mockAPIClient) Authenticate(ctx context.Context, password string) error {
	if m.authenticateFunc != nil {
		return m.authenticateFunc(ctx, password)
	}
	return nil
}

func (m *mockAPIClient) PatchLinuxInstallAppConfigValues(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfigValues, error) {
	if m.patchLinuxInstallAppConfigValuesFunc != nil {
		return m.patchLinuxInstallAppConfigValuesFunc(ctx, values)
	}
	return values, nil
}

func (m *mockAPIClient) GetLinuxInstallationConfig(ctx context.Context) (apitypes.LinuxInstallationConfigResponse, error) {
	if m.getLinuxInstallationConfigFunc != nil {
		return m.getLinuxInstallationConfigFunc(ctx)
	}
	return apitypes.LinuxInstallationConfigResponse{}, nil
}

func (m *mockAPIClient) GetLinuxInstallationStatus(ctx context.Context) (apitypes.Status, error) {
	if m.getLinuxInstallationStatusFunc != nil {
		return m.getLinuxInstallationStatusFunc(ctx)
	}
	return apitypes.Status{}, nil
}

func (m *mockAPIClient) ConfigureLinuxInstallation(ctx context.Context, config apitypes.LinuxInstallationConfig) (apitypes.Status, error) {
	if m.configureLinuxInstallationFunc != nil {
		return m.configureLinuxInstallationFunc(ctx, config)
	}
	return apitypes.Status{}, nil
}

func (m *mockAPIClient) RunLinuxInstallHostPreflights(ctx context.Context) (apitypes.InstallHostPreflightsStatusResponse, error) {
	if m.runLinuxInstallHostPreflightsFunc != nil {
		return m.runLinuxInstallHostPreflightsFunc(ctx)
	}
	return apitypes.InstallHostPreflightsStatusResponse{}, nil
}

func (m *mockAPIClient) GetLinuxInstallHostPreflightsStatus(ctx context.Context) (apitypes.InstallHostPreflightsStatusResponse, error) {
	if m.getLinuxInstallHostPreflightsStatusFunc != nil {
		return m.getLinuxInstallHostPreflightsStatusFunc(ctx)
	}
	return apitypes.InstallHostPreflightsStatusResponse{}, nil
}

func (m *mockAPIClient) SetupLinuxInfra(ctx context.Context, ignoreHostPreflights bool) (apitypes.Infra, error) {
	if m.setupLinuxInfraFunc != nil {
		return m.setupLinuxInfraFunc(ctx, ignoreHostPreflights)
	}
	return apitypes.Infra{}, nil
}

func (m *mockAPIClient) GetLinuxInfraStatus(ctx context.Context) (apitypes.Infra, error) {
	if m.getLinuxInfraStatusFunc != nil {
		return m.getLinuxInfraStatusFunc(ctx)
	}
	return apitypes.Infra{}, nil
}

func (m *mockAPIClient) ProcessLinuxAirgap(ctx context.Context) (apitypes.Airgap, error) {
	if m.processLinuxAirgapFunc != nil {
		return m.processLinuxAirgapFunc(ctx)
	}
	return apitypes.Airgap{}, nil
}

func (m *mockAPIClient) GetLinuxAirgapStatus(ctx context.Context) (apitypes.Airgap, error) {
	if m.getLinuxAirgapStatusFunc != nil {
		return m.getLinuxAirgapStatusFunc(ctx)
	}
	return apitypes.Airgap{}, nil
}

func (m *mockAPIClient) GetLinuxInstallAppConfigValues(ctx context.Context) (apitypes.AppConfigValues, error) {
	if m.getLinuxInstallAppConfigValuesFunc != nil {
		return m.getLinuxInstallAppConfigValuesFunc(ctx)
	}
	return apitypes.AppConfigValues{}, nil
}

func (m *mockAPIClient) TemplateLinuxInstallAppConfig(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfig, error) {
	if m.templateLinuxInstallAppConfigFunc != nil {
		return m.templateLinuxInstallAppConfigFunc(ctx, values)
	}
	return apitypes.AppConfig{}, nil
}

func (m *mockAPIClient) RunLinuxInstallAppPreflights(ctx context.Context) (apitypes.InstallAppPreflightsStatusResponse, error) {
	if m.runLinuxInstallAppPreflightsFunc != nil {
		return m.runLinuxInstallAppPreflightsFunc(ctx)
	}
	return apitypes.InstallAppPreflightsStatusResponse{}, nil
}

func (m *mockAPIClient) GetLinuxInstallAppPreflightsStatus(ctx context.Context) (apitypes.InstallAppPreflightsStatusResponse, error) {
	if m.getLinuxInstallAppPreflightsStatusFunc != nil {
		return m.getLinuxInstallAppPreflightsStatusFunc(ctx)
	}
	return apitypes.InstallAppPreflightsStatusResponse{}, nil
}

func (m *mockAPIClient) InstallLinuxApp(ctx context.Context, ignoreAppPreflights bool) (apitypes.AppInstall, error) {
	if m.installLinuxAppFunc != nil {
		return m.installLinuxAppFunc(ctx, ignoreAppPreflights)
	}
	return apitypes.AppInstall{}, nil
}

func (m *mockAPIClient) GetLinuxAppInstallStatus(ctx context.Context) (apitypes.AppInstall, error) {
	if m.getLinuxAppInstallStatusFunc != nil {
		return m.getLinuxAppInstallStatusFunc(ctx)
	}
	return apitypes.AppInstall{}, nil
}

func (m *mockAPIClient) GetLinuxUpgradeAppConfigValues(ctx context.Context) (apitypes.AppConfigValues, error) {
	if m.getLinuxUpgradeAppConfigValuesFunc != nil {
		return m.getLinuxUpgradeAppConfigValuesFunc(ctx)
	}
	return apitypes.AppConfigValues{}, nil
}

func (m *mockAPIClient) PatchLinuxUpgradeAppConfigValues(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfigValues, error) {
	if m.patchLinuxUpgradeAppConfigValuesFunc != nil {
		return m.patchLinuxUpgradeAppConfigValuesFunc(ctx, values)
	}
	return values, nil
}

func (m *mockAPIClient) TemplateLinuxUpgradeAppConfig(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfig, error) {
	if m.templateLinuxUpgradeAppConfigFunc != nil {
		return m.templateLinuxUpgradeAppConfigFunc(ctx, values)
	}
	return apitypes.AppConfig{}, nil
}

func (m *mockAPIClient) RunLinuxUpgradeAppPreflights(ctx context.Context) (apitypes.UpgradeAppPreflightsStatusResponse, error) {
	if m.runLinuxUpgradeAppPreflightsFunc != nil {
		return m.runLinuxUpgradeAppPreflightsFunc(ctx)
	}
	return apitypes.UpgradeAppPreflightsStatusResponse{}, nil
}

func (m *mockAPIClient) GetLinuxUpgradeAppPreflightsStatus(ctx context.Context) (apitypes.UpgradeAppPreflightsStatusResponse, error) {
	if m.getLinuxUpgradeAppPreflightsStatusFunc != nil {
		return m.getLinuxUpgradeAppPreflightsStatusFunc(ctx)
	}
	return apitypes.UpgradeAppPreflightsStatusResponse{}, nil
}

func (m *mockAPIClient) UpgradeLinuxApp(ctx context.Context, ignoreAppPreflights bool) (apitypes.AppUpgrade, error) {
	if m.upgradeLinuxAppFunc != nil {
		return m.upgradeLinuxAppFunc(ctx, ignoreAppPreflights)
	}
	return apitypes.AppUpgrade{}, nil
}

func (m *mockAPIClient) GetLinuxAppUpgradeStatus(ctx context.Context) (apitypes.AppUpgrade, error) {
	if m.getLinuxAppUpgradeStatusFunc != nil {
		return m.getLinuxAppUpgradeStatusFunc(ctx)
	}
	return apitypes.AppUpgrade{}, nil
}

func (m *mockAPIClient) UpgradeLinuxInfra(ctx context.Context, ignoreHostPreflights bool) (apitypes.Infra, error) {
	if m.upgradeLinuxInfraFunc != nil {
		return m.upgradeLinuxInfraFunc(ctx, ignoreHostPreflights)
	}
	return apitypes.Infra{}, nil
}

func (m *mockAPIClient) GetLinuxUpgradeInfraStatus(ctx context.Context) (apitypes.Infra, error) {
	if m.getLinuxUpgradeInfraStatusFunc != nil {
		return m.getLinuxUpgradeInfraStatusFunc(ctx)
	}
	return apitypes.Infra{}, nil
}

func (m *mockAPIClient) ProcessLinuxUpgradeAirgap(ctx context.Context) (apitypes.Airgap, error) {
	if m.processLinuxUpgradeAirgapFunc != nil {
		return m.processLinuxUpgradeAirgapFunc(ctx)
	}
	return apitypes.Airgap{}, nil
}

func (m *mockAPIClient) GetLinuxUpgradeAirgapStatus(ctx context.Context) (apitypes.Airgap, error) {
	if m.getLinuxUpgradeAirgapStatusFunc != nil {
		return m.getLinuxUpgradeAirgapStatusFunc(ctx)
	}
	return apitypes.Airgap{}, nil
}

func (m *mockAPIClient) RunLinuxUpgradeHostPreflights(ctx context.Context) (apitypes.HostPreflights, error) {
	if m.runLinuxUpgradeHostPreflightsFunc != nil {
		return m.runLinuxUpgradeHostPreflightsFunc(ctx)
	}
	return apitypes.HostPreflights{}, nil
}

func (m *mockAPIClient) GetLinuxUpgradeHostPreflightsStatus(ctx context.Context) (apitypes.HostPreflights, error) {
	if m.getLinuxUpgradeHostPreflightsStatusFunc != nil {
		return m.getLinuxUpgradeHostPreflightsStatusFunc(ctx)
	}
	return apitypes.HostPreflights{}, nil
}

// Kubernetes method implementations (not used in current implementation)
func (m *mockAPIClient) GetKubernetesInstallationConfig(ctx context.Context) (apitypes.KubernetesInstallationConfigResponse, error) {
	if m.getKubernetesInstallationConfigFunc != nil {
		return m.getKubernetesInstallationConfigFunc(ctx)
	}
	return apitypes.KubernetesInstallationConfigResponse{}, nil
}

func (m *mockAPIClient) ConfigureKubernetesInstallation(ctx context.Context, config apitypes.KubernetesInstallationConfig) (apitypes.Status, error) {
	if m.configureKubernetesInstallationFunc != nil {
		return m.configureKubernetesInstallationFunc(ctx, config)
	}
	return apitypes.Status{}, nil
}

func (m *mockAPIClient) GetKubernetesInstallationStatus(ctx context.Context) (apitypes.Status, error) {
	if m.getKubernetesInstallationStatusFunc != nil {
		return m.getKubernetesInstallationStatusFunc(ctx)
	}
	return apitypes.Status{}, nil
}

func (m *mockAPIClient) SetupKubernetesInfra(ctx context.Context) (apitypes.Infra, error) {
	if m.setupKubernetesInfraFunc != nil {
		return m.setupKubernetesInfraFunc(ctx)
	}
	return apitypes.Infra{}, nil
}

func (m *mockAPIClient) GetKubernetesInfraStatus(ctx context.Context) (apitypes.Infra, error) {
	if m.getKubernetesInfraStatusFunc != nil {
		return m.getKubernetesInfraStatusFunc(ctx)
	}
	return apitypes.Infra{}, nil
}

func (m *mockAPIClient) GetKubernetesInstallAppConfigValues(ctx context.Context) (apitypes.AppConfigValues, error) {
	if m.getKubernetesInstallAppConfigValuesFunc != nil {
		return m.getKubernetesInstallAppConfigValuesFunc(ctx)
	}
	return apitypes.AppConfigValues{}, nil
}

func (m *mockAPIClient) PatchKubernetesInstallAppConfigValues(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfigValues, error) {
	if m.patchKubernetesInstallAppConfigValuesFunc != nil {
		return m.patchKubernetesInstallAppConfigValuesFunc(ctx, values)
	}
	return values, nil
}

func (m *mockAPIClient) TemplateKubernetesInstallAppConfig(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfig, error) {
	if m.templateKubernetesInstallAppConfigFunc != nil {
		return m.templateKubernetesInstallAppConfigFunc(ctx, values)
	}
	return apitypes.AppConfig{}, nil
}

func (m *mockAPIClient) RunKubernetesInstallAppPreflights(ctx context.Context) (apitypes.InstallAppPreflightsStatusResponse, error) {
	if m.runKubernetesInstallAppPreflightsFunc != nil {
		return m.runKubernetesInstallAppPreflightsFunc(ctx)
	}
	return apitypes.InstallAppPreflightsStatusResponse{}, nil
}

func (m *mockAPIClient) GetKubernetesInstallAppPreflightsStatus(ctx context.Context) (apitypes.InstallAppPreflightsStatusResponse, error) {
	if m.getKubernetesInstallAppPreflightsStatusFunc != nil {
		return m.getKubernetesInstallAppPreflightsStatusFunc(ctx)
	}
	return apitypes.InstallAppPreflightsStatusResponse{}, nil
}

func (m *mockAPIClient) InstallKubernetesApp(ctx context.Context) (apitypes.AppInstall, error) {
	if m.installKubernetesAppFunc != nil {
		return m.installKubernetesAppFunc(ctx)
	}
	return apitypes.AppInstall{}, nil
}

func (m *mockAPIClient) GetKubernetesAppInstallStatus(ctx context.Context) (apitypes.AppInstall, error) {
	if m.getKubernetesAppInstallStatusFunc != nil {
		return m.getKubernetesAppInstallStatusFunc(ctx)
	}
	return apitypes.AppInstall{}, nil
}

func (m *mockAPIClient) GetKubernetesUpgradeAppConfigValues(ctx context.Context) (apitypes.AppConfigValues, error) {
	if m.getKubernetesUpgradeAppConfigValuesFunc != nil {
		return m.getKubernetesUpgradeAppConfigValuesFunc(ctx)
	}
	return apitypes.AppConfigValues{}, nil
}

func (m *mockAPIClient) PatchKubernetesUpgradeAppConfigValues(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfigValues, error) {
	if m.patchKubernetesUpgradeAppConfigValuesFunc != nil {
		return m.patchKubernetesUpgradeAppConfigValuesFunc(ctx, values)
	}
	return values, nil
}

func (m *mockAPIClient) TemplateKubernetesUpgradeAppConfig(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfig, error) {
	if m.templateKubernetesUpgradeAppConfigFunc != nil {
		return m.templateKubernetesUpgradeAppConfigFunc(ctx, values)
	}
	return apitypes.AppConfig{}, nil
}

// GetKURLMigrationConfig returns empty config (mock implementation).
func (m *mockAPIClient) GetKURLMigrationConfig(ctx context.Context) (apitypes.LinuxInstallationConfigResponse, error) {
	return apitypes.LinuxInstallationConfigResponse{}, nil
}

// StartKURLMigration returns empty response (mock implementation).
func (m *mockAPIClient) StartKURLMigration(ctx context.Context, transferMode string, config *apitypes.LinuxInstallationConfig) (apitypes.StartKURLMigrationResponse, error) {
	return apitypes.StartKURLMigrationResponse{}, nil
}

// GetKURLMigrationStatus returns empty status (mock implementation).
func (m *mockAPIClient) GetKURLMigrationStatus(ctx context.Context) (apitypes.KURLMigrationStatusResponse, error) {
	return apitypes.KURLMigrationStatusResponse{}, nil
}

// Ensure mockAPIClient implements client.Client interface
var _ client.Client = (*mockAPIClient)(nil)
