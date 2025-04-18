package kotsadm

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/replicatedhq/embedded-cluster/kinds/types/join"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// GetJoinCommand determines the IP address of the kotsadm service and makes a request to that IP to get a join command
// for the provided set of roles
func (c *Client) GetJoinCommand(ctx context.Context, cli client.Client, roles []string) (string, error) {
	// determine the IP address and port of the kotsadm service
	svc := &corev1.Service{}
	err := cli.Get(ctx, types.NamespacedName{Name: "kotsadm", Namespace: "kotsadm"}, svc)
	if err != nil {
		return "", fmt.Errorf("unable to get kotsadm service: %w", err)
	}
	kotsadmIP := svc.Spec.ClusterIP
	if kotsadmIP == "" {
		return "", fmt.Errorf("kotsadm service ip was empty")
	}

	if len(svc.Spec.Ports) == 0 {
		return "", fmt.Errorf("kotsadm service ports were empty")
	}
	kotsadmPort := svc.Spec.Ports[0].Port

	// get an auth token for the kotsadm service
	// kots would create this secret if it didn't exist, but we don't need that logic here - the install process should have created it already
	authSecret := &corev1.Secret{}
	err = cli.Get(ctx, types.NamespacedName{Name: "kotsadm-authstring", Namespace: "kotsadm"}, authSecret)
	if err != nil {
		return "", fmt.Errorf("failed to get kotsadm auth slug: %w", err)
	}
	authSlug := string(authSecret.Data["kotsadm-authstring"])

	// build the request body
	body, err := json.Marshal(map[string][]string{
		"roles": roles,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	// send the request
	url := fmt.Sprintf("http://%s:%d/api/v1/embedded-cluster/generate-node-join-command", kotsadmIP, kotsadmPort)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", authSlug)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	fullResponse, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	type commandResponse struct {
		Command []string `json:"command"`
	}
	cmd := commandResponse{}
	if err := json.Unmarshal(fullResponse, &cmd); err != nil {
		logrus.Debugf("failed to decode response %q: %v", string(fullResponse), err)
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return strings.Join(cmd.Command, " "), nil
}
