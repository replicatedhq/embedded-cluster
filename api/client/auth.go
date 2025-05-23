package client

import (
	"bytes"
	"encoding/json"
	"net/http"
)

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
		SessionToken string `json:"sessionToken"`
	}
	err = json.NewDecoder(resp.Body).Decode(&loginResp)
	if err != nil {
		return err
	}

	c.token = loginResp.SessionToken
	return nil
}
