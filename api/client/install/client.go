package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/models"
)

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
		c.httpClient = http.DefaultClient
	}

	return c
}

func (c *client) GetInstall() (*models.Install, error) {
	resp, err := http.Get(c.apiURL)
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

	resp, err := http.Post(c.apiURL+"/phase/set-config", "application/json", bytes.NewBuffer(b))
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
	resp, err := http.Post(c.apiURL+"/phase/start", "application/json", nil)
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
