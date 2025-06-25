package client

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (c *client) GetLinuxInstallationConfig() (types.LinuxInstallationConfig, error) {
	req, err := http.NewRequest("GET", c.apiURL+"/api/linux/install/installation/config", nil)
	if err != nil {
		return types.LinuxInstallationConfig{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.LinuxInstallationConfig{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.LinuxInstallationConfig{}, errorFromResponse(resp)
	}

	var config types.LinuxInstallationConfig
	err = json.NewDecoder(resp.Body).Decode(&config)
	if err != nil {
		return types.LinuxInstallationConfig{}, err
	}

	return config, nil
}

func (c *client) ConfigureLinuxInstallation(config types.LinuxInstallationConfig) (types.Status, error) {
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

func (c *client) GetLinuxInstallationStatus() (types.Status, error) {
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

func (c *client) SetupLinuxInfra() (types.LinuxInfra, error) {
	req, err := http.NewRequest("POST", c.apiURL+"/api/linux/install/infra/setup", nil)
	if err != nil {
		return types.LinuxInfra{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.LinuxInfra{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.LinuxInfra{}, errorFromResponse(resp)
	}

	var infra types.LinuxInfra
	err = json.NewDecoder(resp.Body).Decode(&infra)
	if err != nil {
		return types.LinuxInfra{}, err
	}

	return infra, nil
}

func (c *client) GetLinuxInfraStatus() (types.LinuxInfra, error) {
	req, err := http.NewRequest("GET", c.apiURL+"/api/linux/install/infra/status", nil)
	if err != nil {
		return types.LinuxInfra{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.LinuxInfra{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.LinuxInfra{}, errorFromResponse(resp)
	}

	var infra types.LinuxInfra
	err = json.NewDecoder(resp.Body).Decode(&infra)
	if err != nil {
		return types.LinuxInfra{}, err
	}

	return infra, nil
}

func (c *client) SetLinuxInstallStatus(s types.Status) (types.Status, error) {
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
