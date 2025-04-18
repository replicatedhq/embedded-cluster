package kotsadm

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/replicatedhq/embedded-cluster/kinds/types/join"
)

var _ ClientInterface = (*Client)(nil)

type Client struct{}

// GetJoinToken issues a request to the kots api to get the actual join command
// based on the short token provided by the user.
func (c *Client) GetJoinToken(ctx context.Context, baseURL, shortToken string) (*join.JoinCommandResponse, error) {
	url := fmt.Sprintf("https://%s/api/v1/embedded-cluster/join?token=%s", baseURL, shortToken)
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
