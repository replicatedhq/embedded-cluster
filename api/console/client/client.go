package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/console"
)

type Client interface {
	GetConfig() (*console.Config, error)
	UpsertConfig(config console.Config) (*console.Config, error)
}

type client struct {
	apiURL string
}

func NewClient(apiURL string) Client {
	return &client{
		apiURL: apiURL,
	}
}

func (c *client) GetConfig() (*console.Config, error) {
	resp, err := http.Get(c.apiURL + "/config")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get config: %s", resp.Status)
	}

	var config console.Config
	err = json.NewDecoder(resp.Body).Decode(&config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func (c *client) UpsertConfig(config console.Config) (*console.Config, error) {
	b, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(c.apiURL+"/config", "application/json", bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to update config: %s", resp.Status)
	}

	var updatedConfig console.Config
	err = json.NewDecoder(resp.Body).Decode(&updatedConfig)
	if err != nil {
		return nil, err
	}

	return &updatedConfig, nil
}
