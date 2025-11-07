package install

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/client"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
)

// mockAPIClient is a mock implementation of the client.Client interface for testing
type mockAPIClient struct {
	authenticateFunc                       func(ctx context.Context, password string) error
	patchLinuxInstallAppConfigValuesFunc   func(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfigValues, error)
	getLinuxInstallationConfigFunc         func(ctx context.Context) (apitypes.LinuxInstallationConfigResponse, error)
	getLinuxInstallationStatusFunc         func(ctx context.Context) (apitypes.Status, error)
	configureLinuxInstallationFunc         func(ctx context.Context, config apitypes.LinuxInstallationConfig) (apitypes.Status, error)
	setupLinuxInfraFunc                    func(ctx context.Context, ignoreHostPreflights bool) (apitypes.Infra, error)
	getLinuxInfraStatusFunc                func(ctx context.Context) (apitypes.Infra, error)
	getLinuxInstallAppConfigValuesFunc     func(ctx context.Context) (apitypes.AppConfigValues, error)
	templateLinuxInstallAppConfigFunc      func(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfig, error)
	runLinuxInstallAppPreflightsFunc       func(ctx context.Context) (apitypes.InstallAppPreflightsStatusResponse, error)
	getLinuxInstallAppPreflightsStatusFunc func(ctx context.Context) (apitypes.InstallAppPreflightsStatusResponse, error)
	installLinuxAppFunc                    func(ctx context.Context) (apitypes.AppInstall, error)
	getLinuxAppInstallStatusFunc           func(ctx context.Context) (apitypes.AppInstall, error)
	getLinuxUpgradeAppConfigValuesFunc     func(ctx context.Context) (apitypes.AppConfigValues, error)
	patchLinuxUpgradeAppConfigValuesFunc   func(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfigValues, error)
	templateLinuxUpgradeAppConfigFunc      func(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfig, error)

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

func (m *mockAPIClient) InstallLinuxApp(ctx context.Context) (apitypes.AppInstall, error) {
	if m.installLinuxAppFunc != nil {
		return m.installLinuxAppFunc(ctx)
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

// Ensure mockAPIClient implements client.Client interface
var _ client.Client = (*mockAPIClient)(nil)
