package kotsadm

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/replicatedhq/embedded-cluster/kinds/types/join"
)

var _ ClientInterface = (*Client)(nil)

type Client struct{}

// GetJoinToken issues a request to the kots api to get the actual join command
// based on the short token provided by the user.
func (c *Client) GetJoinToken(ctx context.Context, kotsAPIAddress, shortToken string) (*join.JoinCommandResponse, error) {
	url := fmt.Sprintf("https://%s/api/v1/embedded-cluster/join?token=%s", kotsAPIAddress, shortToken)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create request: %w", err)
	}

	// this will generally be a self-signed certificate created by kurl-proxy
	insecureClient := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	resp, err := insecureClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to get join token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	var command join.JoinCommandResponse
	if err := json.NewDecoder(resp.Body).Decode(&command); err != nil {
		return nil, fmt.Errorf("unable to decode response: %w", err)
	}
	return &command, nil
}

// GetK0sImagesFile fetches the k0s images file from the KOTS API.
// caller is responsible for closing the response body.
func (c *Client) GetK0sImagesFile(ctx context.Context, kotsAPIAddress string) (io.ReadCloser, error) {
	url := fmt.Sprintf("http://%s/api/v1/embedded-cluster/k0s-images", kotsAPIAddress)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create request: %w", err)
	}

	// this will generally be a self-signed certificate created by kurl-proxy
	insecureClient := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	resp, err := insecureClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch k0s images: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code fetching k0s images: %d", resp.StatusCode)
	}
	return resp.Body, nil
}

// GetECCharts fetches the helm charts file from the KOTS API.
// caller is responsible for closing the response body.
func (c *Client) GetECCharts(ctx context.Context, kotsAPIAddress string) (io.ReadCloser, error) {
	url := fmt.Sprintf("http://%s/api/v1/embedded-cluster/charts", kotsAPIAddress)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create request: %w", err)
	}

	// this will generally be a self-signed certificate created by kurl-proxy
	insecureClient := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	resp, err := insecureClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch charts: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code fetching charts: %d", resp.StatusCode)
	}
	return resp.Body, nil
}
