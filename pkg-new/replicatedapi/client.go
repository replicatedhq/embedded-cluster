package replicatedapi

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	kyaml "sigs.k8s.io/yaml"
)

var _ Client = (*client)(nil)

var defaultHTTPClient = newRetryableHTTPClient()

type Client interface {
	SyncLicense(ctx context.Context) (*kotsv1beta1.License, []byte, error)
	GetPendingReleases(ctx context.Context, channelID string, currentSequence int64, opts *PendingReleasesOptions) (*PendingReleasesResponse, error)
}

type client struct {
	replicatedAppURL string
	license          *kotsv1beta1.License
	releaseData      *release.ReleaseData
	clusterID        string
	httpClient       *retryablehttp.Client
}

type ClientOption func(*client)

func WithClusterID(clusterID string) ClientOption {
	return func(c *client) {
		c.clusterID = clusterID
	}
}

func WithHTTPClient(httpClient *retryablehttp.Client) ClientOption {
	return func(c *client) {
		c.httpClient = httpClient
	}
}

func NewClient(replicatedAppURL string, license *kotsv1beta1.License, releaseData *release.ReleaseData, opts ...ClientOption) (Client, error) {
	c := &client{
		replicatedAppURL: replicatedAppURL,
		license:          license,
		releaseData:      releaseData,
		httpClient:       defaultHTTPClient,
	}
	for _, opt := range opts {
		opt(c)
	}
	if _, err := c.getChannelFromLicense(); err != nil {
		return nil, err
	}
	return c, nil
}

// SyncLicense fetches the latest license from the Replicated API
func (c *client) SyncLicense(ctx context.Context) (*kotsv1beta1.License, []byte, error) {
	u := fmt.Sprintf("%s/license/%s", c.replicatedAppURL, c.license.Spec.AppSlug)

	params := url.Values{}
	params.Set("licenseSequence", fmt.Sprintf("%d", c.license.Spec.LicenseSequence))
	if c.releaseData != nil && c.releaseData.ChannelRelease != nil {
		params.Set("selectedChannelId", c.releaseData.ChannelRelease.ChannelID)
	}
	u = fmt.Sprintf("%s?%s", u, params.Encode())

	req, err := c.newRetryableRequest(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/yaml")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read response body: %w", err)
	}

	var licenseResp kotsv1beta1.License
	if err := kyaml.Unmarshal(body, &licenseResp); err != nil {
		return nil, nil, fmt.Errorf("unmarshal license response: %w", err)
	}

	if licenseResp.Spec.LicenseID == "" {
		return nil, nil, fmt.Errorf("license is empty")
	}

	c.license = &licenseResp

	if _, err := c.getChannelFromLicense(); err != nil {
		return nil, nil, fmt.Errorf("get channel from license: %w", err)
	}

	return &licenseResp, body, nil
}

// newRetryableRequest returns a retryablehttp.Request object with kots defaults set, including a User-Agent header.
func (c *client) newRetryableRequest(ctx context.Context, method string, url string, body io.Reader) (*retryablehttp.Request, error) {
	req, err := retryablehttp.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	c.injectHeaders(req.Header)

	return req, nil
}

// injectHeaders injects the basic auth header, user agent header, and reporting info headers into the http.Header.
func (c *client) injectHeaders(header http.Header) {
	header.Set("Authorization", "Basic "+basicAuth(c.license.Spec.LicenseID, c.license.Spec.LicenseID))
	header.Set("User-Agent", fmt.Sprintf("Embedded-Cluster/%s", versions.Version))

	c.injectReportingInfoHeaders(header)
}

func (c *client) getChannelFromLicense() (*kotsv1beta1.Channel, error) {
	if c.releaseData == nil || c.releaseData.ChannelRelease == nil || c.releaseData.ChannelRelease.ChannelID == "" {
		return nil, fmt.Errorf("channel release is empty")
	}
	if c.license == nil || c.license.Spec.LicenseID == "" {
		return nil, fmt.Errorf("license is empty")
	}
	for _, channel := range c.license.Spec.Channels {
		if channel.ChannelID == c.releaseData.ChannelRelease.ChannelID {
			return &channel, nil
		}
	}
	if c.license.Spec.ChannelID == c.releaseData.ChannelRelease.ChannelID {
		return &kotsv1beta1.Channel{
			ChannelID:   c.license.Spec.ChannelID,
			ChannelName: c.license.Spec.ChannelName,
		}, nil
	}
	return nil, fmt.Errorf("channel %s not found in license", c.releaseData.ChannelRelease.ChannelID)
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

// GetPendingReleases fetches pending releases from the Replicated API
func (c *client) GetPendingReleases(ctx context.Context, channelID string, currentSequence int64, opts *PendingReleasesOptions) (*PendingReleasesResponse, error) {
	u := fmt.Sprintf("%s/release/%s/pending", c.replicatedAppURL, c.license.Spec.AppSlug)

	params := url.Values{}
	params.Set("selectedChannelId", channelID)
	params.Set("currentSequence", fmt.Sprintf("%d", currentSequence))
	params.Set("isSemverSupported", fmt.Sprintf("%t", opts.IsSemverSupported))
	if opts.SortOrder != "" {
		params.Set("sortOrder", string(opts.SortOrder))
	}
	u = fmt.Sprintf("%s?%s", u, params.Encode())

	req, err := c.newRetryableRequest(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var pendingReleases PendingReleasesResponse
	if err := kyaml.Unmarshal(body, &pendingReleases); err != nil {
		return nil, fmt.Errorf("unmarshal pending releases response: %w", err)
	}

	return &pendingReleases, nil
}
