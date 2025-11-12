package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

type Client interface {
	Authenticate(ctx context.Context, password string) error
	GetLinuxInstallationConfig(ctx context.Context) (types.LinuxInstallationConfigResponse, error)
	GetLinuxInstallationStatus(ctx context.Context) (types.Status, error)
	ConfigureLinuxInstallation(ctx context.Context, config types.LinuxInstallationConfig) (types.Status, error)
	RunLinuxInstallHostPreflights(ctx context.Context) (types.InstallHostPreflightsStatusResponse, error)
	GetLinuxInstallHostPreflightsStatus(ctx context.Context) (types.InstallHostPreflightsStatusResponse, error)
	SetupLinuxInfra(ctx context.Context, ignoreHostPreflights bool) (types.Infra, error)
	GetLinuxInfraStatus(ctx context.Context) (types.Infra, error)
	ProcessLinuxAirgap(ctx context.Context) (types.Airgap, error)
	GetLinuxAirgapStatus(ctx context.Context) (types.Airgap, error)
	GetLinuxInstallAppConfigValues(ctx context.Context) (types.AppConfigValues, error)
	PatchLinuxInstallAppConfigValues(ctx context.Context, values types.AppConfigValues) (types.AppConfigValues, error)
	TemplateLinuxInstallAppConfig(ctx context.Context, values types.AppConfigValues) (types.AppConfig, error)
	RunLinuxInstallAppPreflights(ctx context.Context) (types.InstallAppPreflightsStatusResponse, error)
	GetLinuxInstallAppPreflightsStatus(ctx context.Context) (types.InstallAppPreflightsStatusResponse, error)
	InstallLinuxApp(ctx context.Context, ignoreAppPreflights bool) (types.AppInstall, error)
	GetLinuxAppInstallStatus(ctx context.Context) (types.AppInstall, error)
	GetLinuxUpgradeAppConfigValues(ctx context.Context) (types.AppConfigValues, error)
	PatchLinuxUpgradeAppConfigValues(ctx context.Context, values types.AppConfigValues) (types.AppConfigValues, error)
	TemplateLinuxUpgradeAppConfig(ctx context.Context, values types.AppConfigValues) (types.AppConfig, error)
	RunLinuxUpgradeAppPreflights(ctx context.Context) (types.UpgradeAppPreflightsStatusResponse, error)
	GetLinuxUpgradeAppPreflightsStatus(ctx context.Context) (types.UpgradeAppPreflightsStatusResponse, error)
	UpgradeLinuxApp(ctx context.Context, ignoreAppPreflights bool) (types.AppUpgrade, error)
	GetLinuxAppUpgradeStatus(ctx context.Context) (types.AppUpgrade, error)
	UpgradeLinuxInfra(ctx context.Context) (types.Infra, error)
	GetLinuxUpgradeInfraStatus(ctx context.Context) (types.Infra, error)
	ProcessLinuxUpgradeAirgap(ctx context.Context) (types.Airgap, error)
	GetLinuxUpgradeAirgapStatus(ctx context.Context) (types.Airgap, error)

	GetKubernetesInstallationConfig(ctx context.Context) (types.KubernetesInstallationConfigResponse, error)
	ConfigureKubernetesInstallation(ctx context.Context, config types.KubernetesInstallationConfig) (types.Status, error)
	GetKubernetesInstallationStatus(ctx context.Context) (types.Status, error)
	SetupKubernetesInfra(ctx context.Context) (types.Infra, error)
	GetKubernetesInfraStatus(ctx context.Context) (types.Infra, error)
	GetKubernetesInstallAppConfigValues(ctx context.Context) (types.AppConfigValues, error)
	PatchKubernetesInstallAppConfigValues(ctx context.Context, values types.AppConfigValues) (types.AppConfigValues, error)
	TemplateKubernetesInstallAppConfig(ctx context.Context, values types.AppConfigValues) (types.AppConfig, error)
	RunKubernetesInstallAppPreflights(ctx context.Context) (types.InstallAppPreflightsStatusResponse, error)
	GetKubernetesInstallAppPreflightsStatus(ctx context.Context) (types.InstallAppPreflightsStatusResponse, error)
	InstallKubernetesApp(ctx context.Context) (types.AppInstall, error)
	GetKubernetesAppInstallStatus(ctx context.Context) (types.AppInstall, error)
	GetKubernetesUpgradeAppConfigValues(ctx context.Context) (types.AppConfigValues, error)
	PatchKubernetesUpgradeAppConfigValues(ctx context.Context, values types.AppConfigValues) (types.AppConfigValues, error)
	TemplateKubernetesUpgradeAppConfig(ctx context.Context, values types.AppConfigValues) (types.AppConfig, error)
}

type client struct {
	apiURL     string
	httpClient *http.Client
	token      string
}

type ClientOption func(*client)

func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *client) {
		c.httpClient = httpClient
	}
}

func WithToken(token string) ClientOption {
	return func(c *client) {
		c.token = token
	}
}

func New(apiURL string, opts ...ClientOption) Client {
	c := &client{
		apiURL: apiURL,
	}
	for _, opt := range opts {
		opt(c)
	}

	if c.httpClient == nil {
		c.httpClient = http.DefaultClient
	}

	return c
}

func setAuthorizationHeader(req *http.Request, token string) {
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func errorFromResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("unexpected response: status=%d", resp.StatusCode)
	}
	var apiError types.APIError
	err = json.Unmarshal(body, &apiError)
	if err != nil {
		return fmt.Errorf("unexpected response: status=%d, body=%q", resp.StatusCode, string(body))
	}
	return &apiError
}
