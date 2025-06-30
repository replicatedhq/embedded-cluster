package client

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (c *client) GetInstallationConfig() (types.InstallationConfig, error) {
	req, err := http.NewRequest("GET", c.apiURL+"/api/linux/install/installation/config", nil)
	if err != nil {
		return types.InstallationConfig{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.InstallationConfig{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.InstallationConfig{}, errorFromResponse(resp)
	}

	var config types.InstallationConfig
	err = json.NewDecoder(resp.Body).Decode(&config)
	if err != nil {
		return types.InstallationConfig{}, err
	}

	return config, nil
}

func (c *client) ConfigureInstallation(config types.InstallationConfig) (types.Status, error) {
	b, err := json.Marshal(config)
	if err != nil {
		return types.Status{}, err
	}

	req, err := http.NewRequest("POST", c.apiURL+"/api/linux/install/installation/configure", bytes.NewBuffer(b))
	if err != nil {
		return types.Status{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.Status{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.Status{}, errorFromResponse(resp)
	}

	var status types.Status
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return types.Status{}, err
	}

	return status, nil
}

func (c *client) GetInstallationStatus() (types.Status, error) {
	req, err := http.NewRequest("GET", c.apiURL+"/api/linux/install/installation/status", nil)
	if err != nil {
		return types.Status{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.Status{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.Status{}, errorFromResponse(resp)
	}

	var status types.Status
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return types.Status{}, err
	}

	return status, nil
}

func (c *client) SetupInfra() (types.Infra, error) {
	req, err := http.NewRequest("POST", c.apiURL+"/api/linux/install/infra/setup", nil)
	if err != nil {
		return types.Infra{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.Infra{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.Infra{}, errorFromResponse(resp)
	}

	var infra types.Infra
	err = json.NewDecoder(resp.Body).Decode(&infra)
	if err != nil {
		return types.Infra{}, err
	}

	return infra, nil
}

func (c *client) GetInfraStatus() (types.Infra, error) {
	req, err := http.NewRequest("GET", c.apiURL+"/api/linux/install/infra/status", nil)
	if err != nil {
		return types.Infra{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.Infra{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.Infra{}, errorFromResponse(resp)
	}

	var infra types.Infra
	err = json.NewDecoder(resp.Body).Decode(&infra)
	if err != nil {
		return types.Infra{}, err
	}

	return infra, nil
}

func (c *client) SetInstallStatus(s types.Status) (types.Status, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return types.Status{}, err
	}

	req, err := http.NewRequest("POST", c.apiURL+"/api/linux/install/status", bytes.NewBuffer(b))
	if err != nil {
		return types.Status{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.Status{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.Status{}, errorFromResponse(resp)
	}

	var status types.Status
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return types.Status{}, err
	}

	return status, nil
}
