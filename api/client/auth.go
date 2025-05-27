package client

import (
	"bytes"
	"encoding/json"
	"net/http"
)

// Login sends a login request to the API server with the provided password and retrieves a session token. The token is stored in the client struct for subsequent requests.
func (c *client) Login(password string) error {
	loginReq := struct {
		Password string `json:"password"`
	}{
		Password: password,
	}

	b, err := json.Marshal(loginReq)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.apiURL+"/api/auth/login", bytes.NewBuffer(b))
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

	var loginResp struct {
		Token string `json:"token"`
	}
	err = json.NewDecoder(resp.Body).Decode(&loginResp)
	if err != nil {
		return err
	}

	c.token = loginResp.Token
	return nil
}
