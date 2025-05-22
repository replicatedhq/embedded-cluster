package install

import (
	"bytes"
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
	GetInstall() (*types.Install, error)
	InstallPhaseSetConfig(config types.InstallationConfig) (*types.Install, error)
}

type client struct {
	apiURL     string
	httpClient *http.Client
}

type ClientOption func(*client)

func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *client) {
		c.httpClient = httpClient
	}
}

func New(apiURL string, opts ...ClientOption) Client {
	c := &client{
		apiURL: apiURL + "/install",
	}
	for _, opt := range opts {
		opt(c)
	}

	if c.httpClient == nil {
		c.httpClient = defaultHTTPClient
	}

	return c
}

func (c *client) GetInstall() (*types.Install, error) {
	req, err := http.NewRequest("GET", c.apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errorFromResponse(resp)
	}

	var install types.Install
	err = json.NewDecoder(resp.Body).Decode(&install)
	if err != nil {
		return nil, err
	}

	return &install, nil
}

func (c *client) InstallPhaseSetConfig(config types.InstallationConfig) (*types.Install, error) {
	b, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.apiURL+"/phase/set-config", bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errorFromResponse(resp)
	}

	var install types.Install
	err = json.NewDecoder(resp.Body).Decode(&install)
	if err != nil {
		return nil, err
	}

	return &install, nil
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
