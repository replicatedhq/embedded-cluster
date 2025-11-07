package client

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

// Authenticate sends a login request to the API server with the provided password and retrieves a
// session token. The token is stored in the client struct for subsequent requests.
func (c *client) Authenticate(ctx context.Context, password string) error {
	b, err := json.Marshal(types.AuthRequest{Password: password})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/api/auth/login", bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errorFromResponse(resp)
	}

	var authResponse types.AuthResponse
	err = json.NewDecoder(resp.Body).Decode(&authResponse)
	if err != nil {
		return err
	}

	c.token = authResponse.Token
	return nil
}
