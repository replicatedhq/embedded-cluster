package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("status=%d, message=%q", e.StatusCode, e.Message)
}

var defaultHTTPClient = &http.Client{
	Transport: &http.Transport{
		Proxy: nil, // This is a local client so no proxy is needed
	},
}

type Client interface {
	Login(password string) error
	GetInstall() (*types.Install, error)
	SetInstallConfig(config types.InstallationConfig) (*types.Install, error)
	SetInstallStatus(status types.InstallationStatus) (*types.Install, error)
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
		c.httpClient = defaultHTTPClient
	}

	return c
}

func errorFromResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("unexpected response: status=%d", resp.StatusCode)
	}
	var apiError APIError
	err = json.Unmarshal(body, &apiError)
	if err != nil {
		return fmt.Errorf("unexpected response: status=%d, body=%q", resp.StatusCode, string(body))
	}
	return &apiError
}
