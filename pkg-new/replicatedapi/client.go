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
	"github.com/replicatedhq/kotskinds/pkg/licensewrapper"
)

var defaultHTTPClient = newRetryableHTTPClient()

// ClientFactory is a function type for creating replicatedapi clients
type ClientFactory func(replicatedAppURL string, license *licensewrapper.LicenseWrapper, releaseData *release.ReleaseData, opts ...ClientOption) (Client, error)

var clientFactory ClientFactory = defaultNewClient

// SetClientFactory sets a custom factory for creating clients (used for testing/mocking)
func SetClientFactory(factory ClientFactory) {
	clientFactory = factory
}

type Client interface {
	SyncLicense(ctx context.Context) (*licensewrapper.LicenseWrapper, []byte, error)
}

type client struct {
	replicatedAppURL string
	license          *licensewrapper.LicenseWrapper
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

func NewClient(replicatedAppURL string, license *licensewrapper.LicenseWrapper, releaseData *release.ReleaseData, opts ...ClientOption) (Client, error) {
	// NewClient creates a new replicatedapi client using the configured factory
	return clientFactory(replicatedAppURL, license, releaseData, opts...)
}

// defaultNewClient is the default implementation of NewClient
func defaultNewClient(replicatedAppURL string, license *licensewrapper.LicenseWrapper, releaseData *release.ReleaseData, opts ...ClientOption) (Client, error) {
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
func (c *client) SyncLicense(ctx context.Context) (*licensewrapper.LicenseWrapper, []byte, error) {
	if c.license == nil {
		return nil, nil, fmt.Errorf("no license configured")
	}

	u := fmt.Sprintf("%s/license/%s", c.replicatedAppURL, c.license.GetAppSlug())

	params := url.Values{}
	params.Set("licenseSequence", fmt.Sprintf("%d", c.license.GetLicenseSequence()))
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

	// Parse response into wrapper (handles both v1beta1 and v1beta2 responses)
	licenseWrapper, err := licensewrapper.LoadLicenseFromBytes(body)
	if err != nil {
		return nil, nil, fmt.Errorf("parse license response: %w", err)
	}

	if licenseWrapper.GetLicenseID() == "" {
		return nil, nil, fmt.Errorf("license is empty")
	}

	c.license = &licenseWrapper

	if _, err := c.getChannelFromLicense(); err != nil {
		return nil, nil, fmt.Errorf("get channel from license: %w", err)
	}

	return &licenseWrapper, body, nil
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
	if c.license == nil {
		return
	}
	licenseID := c.license.GetLicenseID()
	header.Set("Authorization", "Basic "+basicAuth(licenseID, licenseID))
	header.Set("User-Agent", fmt.Sprintf("Embedded-Cluster/%s", versions.Version))

	// Add license version header for v1beta2 licenses
	if c.license.IsV2() {
		header.Set("X-Replicated-License-Version", "v1beta2")
	}

	c.injectReportingInfoHeaders(header)
}

func (c *client) getChannelFromLicense() (*kotsv1beta1.Channel, error) {
	if c.releaseData == nil || c.releaseData.ChannelRelease == nil || c.releaseData.ChannelRelease.ChannelID == "" {
		return nil, fmt.Errorf("channel release is empty")
	}
	if c.license == nil || c.license.GetLicenseID() == "" {
		return nil, fmt.Errorf("license is empty")
	}

	// Check multi-channel licenses first
	channels := c.license.GetChannels()
	for _, channel := range channels {
		if channel.ChannelID == c.releaseData.ChannelRelease.ChannelID {
			return &channel, nil
		}
	}

	// Fallback to legacy single-channel license
	if c.license.GetChannelID() == c.releaseData.ChannelRelease.ChannelID {
		return &kotsv1beta1.Channel{
			ChannelID:   c.license.GetChannelID(),
			ChannelName: c.license.GetChannelName(),
		}, nil
	}

	return nil, fmt.Errorf("channel %s not found in license", c.releaseData.ChannelRelease.ChannelID)
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
