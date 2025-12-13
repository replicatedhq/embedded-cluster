package client

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (c *client) GetLinuxUpgradeAppConfigValues(ctx context.Context) (types.AppConfigValues, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/api/linux/upgrade/app/config/values", nil)
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

func (c *client) PatchLinuxUpgradeAppConfigValues(ctx context.Context, values types.AppConfigValues) (types.AppConfigValues, error) {
	req := types.PatchAppConfigValuesRequest{
		Values: values,
	}
	b, err := json.Marshal(req)
	if err != nil {
		return types.AppConfigValues{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "PATCH", c.apiURL+"/api/linux/upgrade/app/config/values", bytes.NewBuffer(b))
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

func (c *client) UpgradeLinuxApp(ctx context.Context, ignoreAppPreflights bool) (types.AppUpgrade, error) {
	request := types.UpgradeAppRequest{
		IgnoreAppPreflights: ignoreAppPreflights,
	}
	b, err := json.Marshal(request)
	if err != nil {
		return types.AppUpgrade{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/api/linux/upgrade/app/upgrade", bytes.NewBuffer(b))
	if err != nil {
		return types.AppUpgrade{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.AppUpgrade{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.AppUpgrade{}, errorFromResponse(resp)
	}

	var status types.AppUpgrade
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return types.AppUpgrade{}, err
	}

	return status, nil
}

func (c *client) GetLinuxAppUpgradeStatus(ctx context.Context) (types.AppUpgrade, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/api/linux/upgrade/app/status", nil)
	if err != nil {
		return types.AppUpgrade{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.AppUpgrade{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.AppUpgrade{}, errorFromResponse(resp)
	}

	var status types.AppUpgrade
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return types.AppUpgrade{}, err
	}

	return status, nil
}

func (c *client) GetKubernetesUpgradeAppConfigValues(ctx context.Context) (types.AppConfigValues, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/api/kubernetes/upgrade/app/config/values", nil)
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

func (c *client) PatchKubernetesUpgradeAppConfigValues(ctx context.Context, values types.AppConfigValues) (types.AppConfigValues, error) {
	request := types.PatchAppConfigValuesRequest{
		Values: values,
	}

	b, err := json.Marshal(request)
	if err != nil {
		return types.AppConfigValues{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", c.apiURL+"/api/kubernetes/upgrade/app/config/values", bytes.NewBuffer(b))
	if err != nil {
		return types.AppConfigValues{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
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

func (c *client) TemplateLinuxUpgradeAppConfig(ctx context.Context, values types.AppConfigValues) (types.AppConfig, error) {
	request := types.TemplateAppConfigRequest{
		Values: values,
	}

	b, err := json.Marshal(request)
	if err != nil {
		return types.AppConfig{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/api/linux/upgrade/app/config/template", bytes.NewBuffer(b))
	if err != nil {
		return types.AppConfig{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
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

func (c *client) TemplateKubernetesUpgradeAppConfig(ctx context.Context, values types.AppConfigValues) (types.AppConfig, error) {
	request := types.TemplateAppConfigRequest{
		Values: values,
	}

	b, err := json.Marshal(request)
	if err != nil {
		return types.AppConfig{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/api/kubernetes/upgrade/app/config/template", bytes.NewBuffer(b))
	if err != nil {
		return types.AppConfig{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
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

func (c *client) RunLinuxUpgradeAppPreflights(ctx context.Context) (types.UpgradeAppPreflightsStatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/api/linux/upgrade/app-preflights/run", nil)
	if err != nil {
		return types.UpgradeAppPreflightsStatusResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.UpgradeAppPreflightsStatusResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.UpgradeAppPreflightsStatusResponse{}, errorFromResponse(resp)
	}

	var status types.UpgradeAppPreflightsStatusResponse
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return types.UpgradeAppPreflightsStatusResponse{}, err
	}

	return status, nil
}

func (c *client) GetLinuxUpgradeAppPreflightsStatus(ctx context.Context) (types.UpgradeAppPreflightsStatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/api/linux/upgrade/app-preflights/status", nil)
	if err != nil {
		return types.UpgradeAppPreflightsStatusResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.UpgradeAppPreflightsStatusResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.UpgradeAppPreflightsStatusResponse{}, errorFromResponse(resp)
	}

	var status types.UpgradeAppPreflightsStatusResponse
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return types.UpgradeAppPreflightsStatusResponse{}, err
	}

	return status, nil
}

// RunLinuxUpgradeHostPreflights runs host preflight checks before upgrade infrastructure
func (c *client) RunLinuxUpgradeHostPreflights(ctx context.Context) (types.HostPreflights, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/api/linux/upgrade/host-preflights/run", nil)
	if err != nil {
		return types.HostPreflights{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.HostPreflights{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.HostPreflights{}, errorFromResponse(resp)
	}

	var status types.HostPreflights
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return types.HostPreflights{}, err
	}

	return status, nil
}

// GetLinuxUpgradeHostPreflightsStatus gets the current status of upgrade host preflights
func (c *client) GetLinuxUpgradeHostPreflightsStatus(ctx context.Context) (types.HostPreflights, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/api/linux/upgrade/host-preflights/status", nil)
	if err != nil {
		return types.HostPreflights{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.HostPreflights{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.HostPreflights{}, errorFromResponse(resp)
	}

	var status types.HostPreflights
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return types.HostPreflights{}, err
	}

	return status, nil
}

func (c *client) UpgradeLinuxInfra(ctx context.Context, ignoreHostPreflights bool) (types.Infra, error) {
	requestBody := types.LinuxInfraUpgradeRequest{
		IgnoreHostPreflights: ignoreHostPreflights,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return types.Infra{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/api/linux/upgrade/infra/upgrade", bytes.NewReader(body))
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

	var status types.Infra
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return types.Infra{}, err
	}
	return status, nil
}

func (c *client) GetLinuxUpgradeInfraStatus(ctx context.Context) (types.Infra, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/api/linux/upgrade/infra/status", nil)
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

	var status types.Infra
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return types.Infra{}, err
	}
	return status, nil
}

func (c *client) ProcessLinuxUpgradeAirgap(ctx context.Context) (types.Airgap, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/api/linux/upgrade/airgap/process", nil)
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

	var status types.Airgap
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return types.Airgap{}, err
	}
	return status, nil
}

func (c *client) GetLinuxUpgradeAirgapStatus(ctx context.Context) (types.Airgap, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/api/linux/upgrade/airgap/status", nil)
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

	var status types.Airgap
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return types.Airgap{}, err
	}
	return status, nil
}

func (c *client) RunKubernetesUpgradeAppPreflights(ctx context.Context) (types.UpgradeAppPreflightsStatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/api/kubernetes/upgrade/app-preflights/run", nil)
	if err != nil {
		return types.UpgradeAppPreflightsStatusResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.UpgradeAppPreflightsStatusResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.UpgradeAppPreflightsStatusResponse{}, errorFromResponse(resp)
	}

	var status types.UpgradeAppPreflightsStatusResponse
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return types.UpgradeAppPreflightsStatusResponse{}, err
	}

	return status, nil
}

func (c *client) GetKubernetesUpgradeAppPreflightsStatus(ctx context.Context) (types.UpgradeAppPreflightsStatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/api/kubernetes/upgrade/app-preflights/status", nil)
	if err != nil {
		return types.UpgradeAppPreflightsStatusResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.UpgradeAppPreflightsStatusResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.UpgradeAppPreflightsStatusResponse{}, errorFromResponse(resp)
	}

	var status types.UpgradeAppPreflightsStatusResponse
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return types.UpgradeAppPreflightsStatusResponse{}, err
	}

	return status, nil
}
