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
	req, err := http.NewRequest("POST", c.apiURL+"/api/linux/upgrade/app-preflights/run", nil)
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
	req, err := http.NewRequest("GET", c.apiURL+"/api/linux/upgrade/app-preflights/status", nil)
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
