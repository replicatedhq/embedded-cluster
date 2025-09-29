package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

type Client interface {
	Authenticate(password string) error
	GetLinuxInstallationConfig() (types.LinuxInstallationConfigResponse, error)
	GetLinuxInstallationStatus() (types.Status, error)
	ConfigureLinuxInstallation(config types.LinuxInstallationConfig) (types.Status, error)
	SetupLinuxInfra(ignoreHostPreflights bool) (types.Infra, error)
	GetLinuxInfraStatus() (types.Infra, error)
	GetLinuxInstallAppConfigValues() (types.AppConfigValues, error)
	PatchLinuxInstallAppConfigValues(types.AppConfigValues) (types.AppConfigValues, error)
	TemplateLinuxInstallAppConfig(values types.AppConfigValues) (types.AppConfig, error)
	RunLinuxInstallAppPreflights() (types.InstallAppPreflightsStatusResponse, error)
	GetLinuxInstallAppPreflightsStatus() (types.InstallAppPreflightsStatusResponse, error)
	InstallLinuxApp() (types.AppInstall, error)
	GetLinuxAppInstallStatus() (types.AppInstall, error)
	GetLinuxUpgradeAppConfigValues() (types.AppConfigValues, error)
	PatchLinuxUpgradeAppConfigValues(types.AppConfigValues) (types.AppConfigValues, error)
	TemplateLinuxUpgradeAppConfig(values types.AppConfigValues) (types.AppConfig, error)

	GetKubernetesInstallationConfig() (types.KubernetesInstallationConfigResponse, error)
	ConfigureKubernetesInstallation(config types.KubernetesInstallationConfig) (types.Status, error)
	GetKubernetesInstallationStatus() (types.Status, error)
	SetupKubernetesInfra() (types.Infra, error)
	GetKubernetesInfraStatus() (types.Infra, error)
	GetKubernetesInstallAppConfigValues() (types.AppConfigValues, error)
	PatchKubernetesInstallAppConfigValues(types.AppConfigValues) (types.AppConfigValues, error)
	TemplateKubernetesInstallAppConfig(values types.AppConfigValues) (types.AppConfig, error)
	RunKubernetesInstallAppPreflights() (types.InstallAppPreflightsStatusResponse, error)
	GetKubernetesInstallAppPreflightsStatus() (types.InstallAppPreflightsStatusResponse, error)
	InstallKubernetesApp() (types.AppInstall, error)
	GetKubernetesAppInstallStatus() (types.AppInstall, error)
	GetKubernetesUpgradeAppConfigValues() (types.AppConfigValues, error)
	PatchKubernetesUpgradeAppConfigValues(types.AppConfigValues) (types.AppConfigValues, error)
	TemplateKubernetesUpgradeAppConfig(values types.AppConfigValues) (types.AppConfig, error)
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
