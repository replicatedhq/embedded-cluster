package client

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (c *client) GetLinuxInstallationConfig(ctx context.Context) (types.LinuxInstallationConfigResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/api/linux/install/installation/config", nil)
	if err != nil {
		return types.LinuxInstallationConfigResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.LinuxInstallationConfigResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.LinuxInstallationConfigResponse{}, errorFromResponse(resp)
	}

	var configResponse types.LinuxInstallationConfigResponse
	err = json.NewDecoder(resp.Body).Decode(&configResponse)
	if err != nil {
		return types.LinuxInstallationConfigResponse{}, err
	}

	return configResponse, nil
}

func (c *client) ConfigureLinuxInstallation(ctx context.Context, config types.LinuxInstallationConfig) (types.Status, error) {
	b, err := json.Marshal(config)
	if err != nil {
		return types.Status{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/api/linux/install/installation/configure", bytes.NewBuffer(b))
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

func (c *client) GetLinuxInstallationStatus(ctx context.Context) (types.Status, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/api/linux/install/installation/status", nil)
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

func (c *client) RunLinuxInstallHostPreflights(ctx context.Context) (types.InstallHostPreflightsStatusResponse, error) {
	b, err := json.Marshal(types.PostInstallRunHostPreflightsRequest{
		IsUI: false,
	})
	if err != nil {
		return types.InstallHostPreflightsStatusResponse{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/api/linux/install/host-preflights/run", bytes.NewBuffer(b))
	if err != nil {
		return types.InstallHostPreflightsStatusResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.InstallHostPreflightsStatusResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.InstallHostPreflightsStatusResponse{}, errorFromResponse(resp)
	}

	var status types.InstallHostPreflightsStatusResponse
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return types.InstallHostPreflightsStatusResponse{}, err
	}

	return status, nil
}

func (c *client) GetLinuxInstallHostPreflightsStatus(ctx context.Context) (types.InstallHostPreflightsStatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/api/linux/install/host-preflights/status", nil)
	if err != nil {
		return types.InstallHostPreflightsStatusResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.InstallHostPreflightsStatusResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.InstallHostPreflightsStatusResponse{}, errorFromResponse(resp)
	}

	var status types.InstallHostPreflightsStatusResponse
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return types.InstallHostPreflightsStatusResponse{}, err
	}

	return status, nil
}

func (c *client) SetupLinuxInfra(ctx context.Context, ignoreHostPreflights bool) (types.Infra, error) {
	b, err := json.Marshal(types.LinuxInfraSetupRequest{
		IgnoreHostPreflights: ignoreHostPreflights,
	})
	if err != nil {
		return types.Infra{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/api/linux/install/infra/setup", bytes.NewBuffer(b))
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

func (c *client) GetLinuxInfraStatus(ctx context.Context) (types.Infra, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/api/linux/install/infra/status", nil)
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

func (c *client) ProcessLinuxAirgap(ctx context.Context) (types.Airgap, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/api/linux/install/airgap/process", nil)
	if err != nil {
		return types.Airgap{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.Airgap{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.Airgap{}, errorFromResponse(resp)
	}

	var airgap types.Airgap
	err = json.NewDecoder(resp.Body).Decode(&airgap)
	if err != nil {
		return types.Airgap{}, err
	}

	return airgap, nil
}

func (c *client) GetLinuxAirgapStatus(ctx context.Context) (types.Airgap, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/api/linux/install/airgap/status", nil)
	if err != nil {
		return types.Airgap{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.Airgap{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.Airgap{}, errorFromResponse(resp)
	}

	var airgap types.Airgap
	err = json.NewDecoder(resp.Body).Decode(&airgap)
	if err != nil {
		return types.Airgap{}, err
	}

	return airgap, nil
}

func (c *client) GetKubernetesInstallationConfig(ctx context.Context) (types.KubernetesInstallationConfigResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/api/kubernetes/install/installation/config", nil)
	if err != nil {
		return types.KubernetesInstallationConfigResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.KubernetesInstallationConfigResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.KubernetesInstallationConfigResponse{}, errorFromResponse(resp)
	}

	var configResponse types.KubernetesInstallationConfigResponse
	err = json.NewDecoder(resp.Body).Decode(&configResponse)
	if err != nil {
		return types.KubernetesInstallationConfigResponse{}, err
	}

	return configResponse, nil
}

func (c *client) ConfigureKubernetesInstallation(ctx context.Context, config types.KubernetesInstallationConfig) (types.Status, error) {
	b, err := json.Marshal(config)
	if err != nil {
		return types.Status{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/api/kubernetes/install/installation/configure", bytes.NewBuffer(b))
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

func (c *client) GetKubernetesInstallationStatus(ctx context.Context) (types.Status, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/api/kubernetes/install/installation/status", nil)
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

func (c *client) SetupKubernetesInfra(ctx context.Context) (types.Infra, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/api/kubernetes/install/infra/setup", nil)
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

func (c *client) GetKubernetesInfraStatus(ctx context.Context) (types.Infra, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/api/kubernetes/install/infra/status", nil)
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

func (c *client) GetLinuxInstallAppConfigValues(ctx context.Context) (types.AppConfigValues, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/api/linux/install/app/config/values", nil)
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

func (c *client) PatchLinuxInstallAppConfigValues(ctx context.Context, values types.AppConfigValues) (types.AppConfigValues, error) {
	req := types.PatchAppConfigValuesRequest{
		Values: values,
	}
	b, err := json.Marshal(req)
	if err != nil {
		return types.AppConfigValues{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "PATCH", c.apiURL+"/api/linux/install/app/config/values", bytes.NewBuffer(b))
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

func (c *client) GetKubernetesInstallAppConfigValues(ctx context.Context) (types.AppConfigValues, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/api/kubernetes/install/app/config/values", nil)
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

func (c *client) PatchKubernetesInstallAppConfigValues(ctx context.Context, values types.AppConfigValues) (types.AppConfigValues, error) {
	request := types.PatchAppConfigValuesRequest{
		Values: values,
	}

	b, err := json.Marshal(request)
	if err != nil {
		return types.AppConfigValues{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "PATCH", c.apiURL+"/api/kubernetes/install/app/config/values", bytes.NewBuffer(b))
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

func (c *client) TemplateLinuxInstallAppConfig(ctx context.Context, values types.AppConfigValues) (types.AppConfig, error) {
	request := types.TemplateAppConfigRequest{
		Values: values,
	}

	b, err := json.Marshal(request)
	if err != nil {
		return types.AppConfig{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/api/linux/install/app/config/template", bytes.NewBuffer(b))
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

func (c *client) TemplateKubernetesInstallAppConfig(ctx context.Context, values types.AppConfigValues) (types.AppConfig, error) {
	request := types.TemplateAppConfigRequest{
		Values: values,
	}

	b, err := json.Marshal(request)
	if err != nil {
		return types.AppConfig{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/api/kubernetes/install/app/config/template", bytes.NewBuffer(b))
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

func (c *client) RunLinuxInstallAppPreflights(ctx context.Context) (types.InstallAppPreflightsStatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/api/linux/install/app-preflights/run", nil)
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

func (c *client) GetLinuxInstallAppPreflightsStatus(ctx context.Context) (types.InstallAppPreflightsStatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/api/linux/install/app-preflights/status", nil)
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

func (c *client) RunKubernetesInstallAppPreflights(ctx context.Context) (types.InstallAppPreflightsStatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/api/kubernetes/install/app-preflights/run", nil)
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

func (c *client) GetKubernetesInstallAppPreflightsStatus(ctx context.Context) (types.InstallAppPreflightsStatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/api/kubernetes/install/app-preflights/status", nil)
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

func (c *client) InstallLinuxApp(ctx context.Context, ignoreAppPreflights bool) (types.AppInstall, error) {
	request := types.InstallAppRequest{
		IgnoreAppPreflights: ignoreAppPreflights,
	}
	b, err := json.Marshal(request)
	if err != nil {
		return types.AppInstall{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/api/linux/install/app/install", bytes.NewBuffer(b))
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

func (c *client) GetLinuxAppInstallStatus(ctx context.Context) (types.AppInstall, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/api/linux/install/app/status", nil)
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

func (c *client) InstallKubernetesApp(ctx context.Context) (types.AppInstall, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/api/kubernetes/install/app/install", nil)
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

func (c *client) GetKubernetesAppInstallStatus(ctx context.Context) (types.AppInstall, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/api/kubernetes/install/app/status", nil)
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
