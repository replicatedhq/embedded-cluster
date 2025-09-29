package client

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (c *client) GetLinuxUpgradeAppConfigValues() (types.AppConfigValues, error) {
	req, err := http.NewRequest("GET", c.apiURL+"/api/linux/upgrade/app/config/values", nil)
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

func (c *client) PatchLinuxUpgradeAppConfigValues(values types.AppConfigValues) (types.AppConfigValues, error) {
	req := types.PatchAppConfigValuesRequest{
		Values: values,
	}
	b, err := json.Marshal(req)
	if err != nil {
		return types.AppConfigValues{}, err
	}

	httpReq, err := http.NewRequest("PATCH", c.apiURL+"/api/linux/upgrade/app/config/values", bytes.NewBuffer(b))
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

func (c *client) GetKubernetesUpgradeAppConfigValues() (types.AppConfigValues, error) {
	req, err := http.NewRequest("GET", c.apiURL+"/api/kubernetes/upgrade/app/config/values", nil)
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

func (c *client) PatchKubernetesUpgradeAppConfigValues(values types.AppConfigValues) (types.AppConfigValues, error) {
	request := types.PatchAppConfigValuesRequest{
		Values: values,
	}

	b, err := json.Marshal(request)
	if err != nil {
		return types.AppConfigValues{}, err
	}

	req, err := http.NewRequest("PATCH", c.apiURL+"/api/kubernetes/upgrade/app/config/values", bytes.NewBuffer(b))
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

func (c *client) TemplateLinuxUpgradeAppConfig(values types.AppConfigValues) (types.AppConfig, error) {
	request := types.TemplateAppConfigRequest{
		Values: values,
	}

	b, err := json.Marshal(request)
	if err != nil {
		return types.AppConfig{}, err
	}

	req, err := http.NewRequest("POST", c.apiURL+"/api/linux/upgrade/app/config/template", bytes.NewBuffer(b))
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

func (c *client) TemplateKubernetesUpgradeAppConfig(values types.AppConfigValues) (types.AppConfig, error) {
	request := types.TemplateAppConfigRequest{
		Values: values,
	}

	b, err := json.Marshal(request)
	if err != nil {
		return types.AppConfig{}, err
	}

	req, err := http.NewRequest("POST", c.apiURL+"/api/kubernetes/upgrade/app/config/template", bytes.NewBuffer(b))
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
