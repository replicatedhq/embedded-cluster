package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/models"
)

var defaultHTTPClient = &http.Client{
	Transport: &http.Transport{
		Proxy: nil, // This is a local client so no proxy is needed
	},
}

type Client interface {
	GetInstall() (*models.Install, error)
	InstallPhaseSetConfig(config models.InstallationConfig) (*models.Install, error)
	InstallPhaseStart() (*models.Install, error)
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

func (c *client) GetInstall() (*models.Install, error) {
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
		return nil, fmt.Errorf("failed to get config: %s", resp.Status)
	}

	var install models.Install
	err = json.NewDecoder(resp.Body).Decode(&install)
	if err != nil {
		return nil, err
	}

	return &install, nil
}

func (c *client) InstallPhaseSetConfig(config models.InstallationConfig) (*models.Install, error) {
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
		return nil, fmt.Errorf("failed to update config: %s", resp.Status)
	}

	var install models.Install
	err = json.NewDecoder(resp.Body).Decode(&install)
	if err != nil {
		return nil, err
	}

	return &install, nil
}

func (c *client) InstallPhaseStart() (*models.Install, error) {
	req, err := http.NewRequest("POST", c.apiURL+"/phase/start", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to update config: %s", resp.Status)
	}

	var install models.Install
	err = json.NewDecoder(resp.Body).Decode(&install)
	if err != nil {
		return nil, err
	}

	return &install, nil
}
