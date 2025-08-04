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

func (c *client) GetLinuxAppConfigValues() (types.AppConfigValues, error) {
	req, err := http.NewRequest("GET", c.apiURL+"/api/linux/install/app/config/values", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errorFromResponse(resp)
	}

	var response types.AppConfigValuesResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	return response.Values, nil
}

func (c *client) PatchLinuxAppConfigValues(values types.AppConfigValues) (types.AppConfigValues, error) {
	req := types.PatchAppConfigValuesRequest{
		Values: values,
	}
	b, err := json.Marshal(req)
	if err != nil {
		return types.AppConfigValues{}, err
	}

	httpReq, err := http.NewRequest("PATCH", c.apiURL+"/api/linux/install/app/config/values", bytes.NewBuffer(b))
	if err != nil {
		return types.AppConfigValues{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(httpReq, c.token)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return types.AppConfigValues{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.AppConfigValues{}, errorFromResponse(resp)
	}

	var config types.AppConfigValuesResponse
	err = json.NewDecoder(resp.Body).Decode(&config)
	if err != nil {
		return types.AppConfigValues{}, err
	}

	return config.Values, nil
}

func (c *client) GetKubernetesAppConfigValues() (types.AppConfigValues, error) {
	req, err := http.NewRequest("GET", c.apiURL+"/api/kubernetes/install/app/config/values", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errorFromResponse(resp)
	}

	var response types.AppConfigValuesResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	return response.Values, nil
}

func (c *client) PatchKubernetesAppConfigValues(values types.AppConfigValues) (types.AppConfigValues, error) {
	request := types.PatchAppConfigValuesRequest{
		Values: values,
	}

	b, err := json.Marshal(request)
	if err != nil {
		return types.AppConfigValues{}, err
	}

	httpReq, err := http.NewRequest("PATCH", c.apiURL+"/api/kubernetes/install/app/config/values", bytes.NewBuffer(b))
	if err != nil {
		return types.AppConfigValues{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(httpReq, c.token)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return types.AppConfigValues{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.AppConfigValues{}, errorFromResponse(resp)
	}

	var config types.AppConfigValuesResponse
	err = json.NewDecoder(resp.Body).Decode(&config)
	if err != nil {
		return types.AppConfigValues{}, err
	}

	return config.Values, nil
}

func (c *client) TemplateLinuxAppConfig(values types.AppConfigValues) (types.AppConfig, error) {
	request := types.TemplateAppConfigRequest{
		Values: values,
	}

	b, err := json.Marshal(request)
	if err != nil {
		return types.AppConfig{}, err
	}

	httpReq, err := http.NewRequest("POST", c.apiURL+"/api/linux/install/app/config/template", bytes.NewBuffer(b))
	if err != nil {
		return types.AppConfig{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(httpReq, c.token)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return types.AppConfig{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.AppConfig{}, errorFromResponse(resp)
	}

	var config types.AppConfig
	err = json.NewDecoder(resp.Body).Decode(&config)
	if err != nil {
		return types.AppConfig{}, err
	}

	return config, nil
}

func (c *client) TemplateKubernetesAppConfig(values types.AppConfigValues) (types.AppConfig, error) {
	request := types.TemplateAppConfigRequest{
		Values: values,
	}

	b, err := json.Marshal(request)
	if err != nil {
		return types.AppConfig{}, err
	}

	httpReq, err := http.NewRequest("POST", c.apiURL+"/api/kubernetes/install/app/config/template", bytes.NewBuffer(b))
	if err != nil {
		return types.AppConfig{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(httpReq, c.token)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return types.AppConfig{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.AppConfig{}, errorFromResponse(resp)
	}

	var config types.AppConfig
	err = json.NewDecoder(resp.Body).Decode(&config)
	if err != nil {
		return types.AppConfig{}, err
	}

	return config, nil
}

func (c *client) RunLinuxAppPreflights() (types.InstallAppPreflightsStatusResponse, error) {
	req, err := http.NewRequest("POST", c.apiURL+"/api/linux/install/app-preflights/run", nil)
	if err != nil {
		return types.InstallAppPreflightsStatusResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.InstallAppPreflightsStatusResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.InstallAppPreflightsStatusResponse{}, errorFromResponse(resp)
	}

	var status types.InstallAppPreflightsStatusResponse
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return types.InstallAppPreflightsStatusResponse{}, err
	}

	return status, nil
}

func (c *client) GetLinuxAppPreflightsStatus() (types.InstallAppPreflightsStatusResponse, error) {
	req, err := http.NewRequest("GET", c.apiURL+"/api/linux/install/app-preflights/status", nil)
	if err != nil {
		return types.InstallAppPreflightsStatusResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.InstallAppPreflightsStatusResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.InstallAppPreflightsStatusResponse{}, errorFromResponse(resp)
	}

	var status types.InstallAppPreflightsStatusResponse
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return types.InstallAppPreflightsStatusResponse{}, err
	}

	return status, nil
}

func (c *client) RunKubernetesAppPreflights() (types.InstallAppPreflightsStatusResponse, error) {
	req, err := http.NewRequest("POST", c.apiURL+"/api/kubernetes/install/app-preflights/run", nil)
	if err != nil {
		return types.InstallAppPreflightsStatusResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.InstallAppPreflightsStatusResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.InstallAppPreflightsStatusResponse{}, errorFromResponse(resp)
	}

	var status types.InstallAppPreflightsStatusResponse
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return types.InstallAppPreflightsStatusResponse{}, err
	}

	return status, nil
}

func (c *client) GetKubernetesAppPreflightsStatus() (types.InstallAppPreflightsStatusResponse, error) {
	req, err := http.NewRequest("GET", c.apiURL+"/api/kubernetes/install/app-preflights/status", nil)
	if err != nil {
		return types.InstallAppPreflightsStatusResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.InstallAppPreflightsStatusResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.InstallAppPreflightsStatusResponse{}, errorFromResponse(resp)
	}

	var status types.InstallAppPreflightsStatusResponse
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return types.InstallAppPreflightsStatusResponse{}, err
	}

	return status, nil
}

func (c *client) InstallLinuxApp() (types.AppInstall, error) {
	req, err := http.NewRequest("POST", c.apiURL+"/api/linux/install/app/install", nil)
	if err != nil {
		return types.AppInstall{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.AppInstall{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.AppInstall{}, errorFromResponse(resp)
	}

	var appInstall types.AppInstall
	err = json.NewDecoder(resp.Body).Decode(&appInstall)
	if err != nil {
		return types.AppInstall{}, err
	}

	return appInstall, nil
}

func (c *client) GetLinuxAppInstallStatus() (types.AppInstall, error) {
	req, err := http.NewRequest("GET", c.apiURL+"/api/linux/install/app/status", nil)
	if err != nil {
		return types.AppInstall{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.AppInstall{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.AppInstall{}, errorFromResponse(resp)
	}

	var appInstall types.AppInstall
	err = json.NewDecoder(resp.Body).Decode(&appInstall)
	if err != nil {
		return types.AppInstall{}, err
	}

	return appInstall, nil
}

func (c *client) InstallKubernetesApp() (types.AppInstall, error) {
	req, err := http.NewRequest("POST", c.apiURL+"/api/kubernetes/install/app/install", nil)
	if err != nil {
		return types.AppInstall{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.AppInstall{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.AppInstall{}, errorFromResponse(resp)
	}

	var appInstall types.AppInstall
	err = json.NewDecoder(resp.Body).Decode(&appInstall)
	if err != nil {
		return types.AppInstall{}, err
	}

	return appInstall, nil
}

func (c *client) GetKubernetesAppInstallStatus() (types.AppInstall, error) {
	req, err := http.NewRequest("GET", c.apiURL+"/api/kubernetes/install/app/status", nil)
	if err != nil {
		return types.AppInstall{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.AppInstall{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.AppInstall{}, errorFromResponse(resp)
	}

	var appInstall types.AppInstall
	err = json.NewDecoder(resp.Body).Decode(&appInstall)
	if err != nil {
		return types.AppInstall{}, err
	}

	return appInstall, nil
}
