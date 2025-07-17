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
	GetLinuxInstallationConfig() (types.LinuxInstallationConfig, error)
	GetLinuxInstallationStatus() (types.Status, error)
	ConfigureLinuxInstallation(config types.LinuxInstallationConfig) (types.Status, error)
	SetupLinuxInfra(ignoreHostPreflights bool) (types.Infra, error)
	GetLinuxInfraStatus() (types.Infra, error)
	GetLinuxAppConfig() (types.AppConfig, error)
	GetLinuxAppConfigValues() (types.AppConfigValues, error)
	PatchLinuxAppConfigValues(types.AppConfigValues) (types.AppConfigValues, error)

	GetKubernetesInstallationConfig() (types.KubernetesInstallationConfig, error)
	ConfigureKubernetesInstallation(config types.KubernetesInstallationConfig) (types.Status, error)
	GetKubernetesInstallationStatus() (types.Status, error)
	SetupKubernetesInfra() (types.Infra, error)
	GetKubernetesInfraStatus() (types.Infra, error)
	GetKubernetesAppConfig() (types.AppConfig, error)
	GetKubernetesAppConfigValues() (types.AppConfigValues, error)
	PatchKubernetesAppConfigValues(types.AppConfigValues) (types.AppConfigValues, error)
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
