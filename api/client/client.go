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
	GetInstallationConfig() (types.InstallationConfig, error)
	GetInstallationStatus() (types.Status, error)
	ConfigureInstallation(config types.InstallationConfig) (types.Status, error)
	SetupInfra() (types.Infra, error)
	GetInfraStatus() (types.Infra, error)
	SetInstallStatus(status types.Status) (types.Status, error)
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
