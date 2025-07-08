package client

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/types"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
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

func (c *client) SetupLinuxInfra(ignoreHostPreflights bool) (types.Infra, error) {
	b, err := json.Marshal(types.LinuxInfraSetupRequest{
		IgnoreHostPreflights: ignoreHostPreflights,
	})
	if err != nil {
		return types.Infra{}, err
	}

	req, err := http.NewRequest("POST", c.apiURL+"/api/linux/install/infra/setup", bytes.NewBuffer(b))
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

func (c *client) GetLinuxInfraStatus() (types.Infra, error) {
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

func (c *client) GetKubernetesInstallationConfig() (types.KubernetesInstallationConfig, error) {
	req, err := http.NewRequest("GET", c.apiURL+"/api/kubernetes/install/installation/config", nil)
	if err != nil {
		return types.KubernetesInstallationConfig{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.KubernetesInstallationConfig{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.KubernetesInstallationConfig{}, errorFromResponse(resp)
	}

	var config types.KubernetesInstallationConfig
	err = json.NewDecoder(resp.Body).Decode(&config)
	if err != nil {
		return types.KubernetesInstallationConfig{}, err
	}

	return config, nil
}

func (c *client) ConfigureKubernetesInstallation(config types.KubernetesInstallationConfig) (types.Status, error) {
	b, err := json.Marshal(config)
	if err != nil {
		return types.Status{}, err
	}

	req, err := http.NewRequest("POST", c.apiURL+"/api/kubernetes/install/installation/configure", bytes.NewBuffer(b))
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

func (c *client) GetKubernetesInstallationStatus() (types.Status, error) {
	req, err := http.NewRequest("GET", c.apiURL+"/api/kubernetes/install/installation/status", nil)
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

func (c *client) SetupKubernetesInfra() (types.Infra, error) {
	req, err := http.NewRequest("POST", c.apiURL+"/api/kubernetes/install/infra/setup", nil)
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

func (c *client) GetKubernetesInfraStatus() (types.Infra, error) {
	req, err := http.NewRequest("GET", c.apiURL+"/api/kubernetes/install/infra/status", nil)
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

func (c *client) GetLinuxAppConfig() (kotsv1beta1.Config, error) {
	req, err := http.NewRequest("GET", c.apiURL+"/api/linux/install/app/config", nil)
	if err != nil {
		return kotsv1beta1.Config{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return kotsv1beta1.Config{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return kotsv1beta1.Config{}, errorFromResponse(resp)
	}

	var config kotsv1beta1.Config
	err = json.NewDecoder(resp.Body).Decode(&config)
	if err != nil {
		return kotsv1beta1.Config{}, err
	}

	return config, nil
}

func (c *client) SetLinuxAppConfig(config kotsv1beta1.Config) (kotsv1beta1.Config, error) {
	b, err := json.Marshal(config)
	if err != nil {
		return kotsv1beta1.Config{}, err
	}

	req, err := http.NewRequest("POST", c.apiURL+"/api/linux/install/app/configure", bytes.NewBuffer(b))
	if err != nil {
		return kotsv1beta1.Config{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return kotsv1beta1.Config{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return kotsv1beta1.Config{}, errorFromResponse(resp)
	}

	var storedConfig kotsv1beta1.Config
	err = json.NewDecoder(resp.Body).Decode(&storedConfig)
	if err != nil {
		return kotsv1beta1.Config{}, err
	}

	return storedConfig, nil
}

func (c *client) GetKubernetesAppConfig() (kotsv1beta1.Config, error) {
	req, err := http.NewRequest("GET", c.apiURL+"/api/kubernetes/install/app/config", nil)
	if err != nil {
		return kotsv1beta1.Config{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return kotsv1beta1.Config{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return kotsv1beta1.Config{}, errorFromResponse(resp)
	}

	var config kotsv1beta1.Config
	err = json.NewDecoder(resp.Body).Decode(&config)
	if err != nil {
		return kotsv1beta1.Config{}, err
	}

	return config, nil
}

func (c *client) SetKubernetesAppConfig(config kotsv1beta1.Config) (kotsv1beta1.Config, error) {
	b, err := json.Marshal(config)
	if err != nil {
		return kotsv1beta1.Config{}, err
	}

	req, err := http.NewRequest("POST", c.apiURL+"/api/kubernetes/install/app/configure", bytes.NewBuffer(b))
	if err != nil {
		return kotsv1beta1.Config{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return kotsv1beta1.Config{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return kotsv1beta1.Config{}, errorFromResponse(resp)
	}

	var storedConfig kotsv1beta1.Config
	err = json.NewDecoder(resp.Body).Decode(&storedConfig)
	if err != nil {
		return kotsv1beta1.Config{}, err
	}

	return storedConfig, nil
}
