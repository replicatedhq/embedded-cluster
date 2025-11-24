package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

// GetKURLMigrationConfig returns the installation configuration with kURL values, EC defaults, and resolved values.
func (c *client) GetKURLMigrationConfig(ctx context.Context) (types.LinuxInstallationConfigResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/api/linux/kurl-migration/config", c.apiURL), nil)
	if err != nil {
		return types.LinuxInstallationConfigResponse{}, fmt.Errorf("create request: %w", err)
	}
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.LinuxInstallationConfigResponse{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.LinuxInstallationConfigResponse{}, errorFromResponse(resp)
	}

	var result types.LinuxInstallationConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return types.LinuxInstallationConfigResponse{}, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}

// StartKURLMigration starts a new kURL to EC migration.
func (c *client) StartKURLMigration(ctx context.Context, transferMode string, config *types.LinuxInstallationConfig) (types.StartKURLMigrationResponse, error) {
	requestBody := types.StartKURLMigrationRequest{
		TransferMode: types.TransferMode(transferMode),
		Config:       config,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return types.StartKURLMigrationResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/api/linux/kurl-migration/start", c.apiURL), bytes.NewReader(body))
	if err != nil {
		return types.StartKURLMigrationResponse{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.StartKURLMigrationResponse{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.StartKURLMigrationResponse{}, errorFromResponse(resp)
	}

	var result types.StartKURLMigrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return types.StartKURLMigrationResponse{}, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}

// GetKURLMigrationStatus returns the current status of the migration.
func (c *client) GetKURLMigrationStatus(ctx context.Context) (types.KURLMigrationStatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/api/linux/kurl-migration/status", c.apiURL), nil)
	if err != nil {
		return types.KURLMigrationStatusResponse{}, fmt.Errorf("create request: %w", err)
	}
	setAuthorizationHeader(req, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.KURLMigrationStatusResponse{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.KURLMigrationStatusResponse{}, errorFromResponse(resp)
	}

	var result types.KURLMigrationStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return types.KURLMigrationStatusResponse{}, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}
