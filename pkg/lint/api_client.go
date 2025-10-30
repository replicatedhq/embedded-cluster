package lint

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// APIClient handles communication with the Replicated API
type APIClient struct {
	apiToken  string
	apiOrigin string
	appID     string
	client    *http.Client
	verbose   bool
}

// NewAPIClient creates a new API client
func NewAPIClient(apiToken, apiOrigin, appID string) *APIClient {
	return &APIClient{
		apiToken:  apiToken,
		apiOrigin: strings.TrimSuffix(apiOrigin, "/"),
		appID:     appID,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		verbose: false,
	}
}

// SetVerbose enables or disables verbose mode
func (c *APIClient) SetVerbose(verbose bool) {
	c.verbose = verbose
}

// logConfiguration logs the detected configuration (for verbose mode)
func (c *APIClient) logConfiguration() {
	fmt.Fprintf(os.Stderr, "Environment configuration:\n")
	fmt.Fprintf(os.Stderr, "  REPLICATED_API_ORIGIN: %s\n", c.apiOrigin)
	fmt.Fprintf(os.Stderr, "  REPLICATED_APP: %s\n", c.appID)
	if c.apiToken != "" {
		fmt.Fprintf(os.Stderr, "  REPLICATED_API_TOKEN: <set>\n")
	} else {
		fmt.Fprintf(os.Stderr, "  REPLICATED_API_TOKEN: <not set>\n")
	}
}

// isConfigured checks if the API client has necessary configuration
func (c *APIClient) isConfigured() bool {
	return c.apiToken != "" && c.apiOrigin != "" && c.appID != ""
}

// getMissingConfig returns a list of missing configuration items
func (c *APIClient) getMissingConfig() []string {
	var missing []string
	if c.apiToken == "" {
		missing = append(missing, "REPLICATED_API_TOKEN")
	}
	if c.apiOrigin == "" {
		missing = append(missing, "REPLICATED_API_ORIGIN")
	}
	if c.appID == "" {
		missing = append(missing, "REPLICATED_APP")
	}
	return missing
}

// CustomDomainsResponse represents the API response for custom domains
type CustomDomainsResponse struct {
	Domains []DomainInfo `json:"domains"`
}

// DomainInfo represents a single domain configuration
type DomainInfo struct {
	Domain string `json:"domain"`
	Type   string `json:"type"` // replicated_app, proxy_registry, replicated_registry
}

// GetCustomDomains fetches the custom domains configured for the app
func (c *APIClient) GetCustomDomains() ([]string, error) {
	if !c.isConfigured() {
		missing := c.getMissingConfig()
		return nil, fmt.Errorf("API client not configured - missing: %v", missing)
	}

	// Try to fetch custom domains from the channel releases endpoint first
	// This is more likely to have the domain information
	if c.verbose {
		fmt.Fprintf(os.Stderr, "Starting custom domain validation\n")
	}

	domains, err := c.getDomainsFromChannelReleases()
	if err == nil && len(domains) > 0 {
		if c.verbose {
			fmt.Fprintf(os.Stderr, "Successfully fetched %d custom domain(s) from channel releases\n", len(domains))
			for _, d := range domains {
				fmt.Fprintf(os.Stderr, "  - %s\n", d)
			}
		}
		return domains, nil
	}

	// If that doesn't work, try a direct custom domains endpoint
	// Note: This endpoint path might need adjustment based on actual API
	url := fmt.Sprintf("%s/v3/app/%s/custom-hostnames", c.apiOrigin, c.appID)

	if c.verbose {
		fmt.Fprintf(os.Stderr, "Attempting to fetch custom domains from: %s\n", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for app %q: %w", c.appID, err)
	}

	req.Header.Set("Authorization", c.apiToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request for app %q: %w", c.appID, err)
	}
	defer resp.Body.Close()

	if c.verbose {
		fmt.Fprintf(os.Stderr, "Response status: %d %s\n", resp.StatusCode, resp.Status)
	}

	if resp.StatusCode == http.StatusNotFound {
		if c.verbose {
			fmt.Fprintf(os.Stderr, "Endpoint not found, trying alternate endpoint structure\n")
		}
		// Try alternate endpoint structure
		return c.getDomainsFromApp()
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d for app %q: %s", resp.StatusCode, c.appID, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var response CustomDomainsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		// Try to parse as array of strings directly
		var domainStrings []string
		if err2 := json.Unmarshal(body, &domainStrings); err2 == nil {
			return domainStrings, nil
		}
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract domain strings from the response
	var result []string
	for _, d := range response.Domains {
		if d.Domain != "" {
			result = append(result, d.Domain)
		}
	}

	return result, nil
}

// ChannelReleasesResponse represents the API response for channel releases
type ChannelReleasesResponse struct {
	ChannelReleases []ChannelRelease `json:"channel_releases"`
}

// ChannelRelease represents a release in a channel
type ChannelRelease struct {
	ChannelID       string   `json:"channel_id"`
	ReleaseSequence int      `json:"release_sequence"`
	VersionLabel    string   `json:"version_label"`
	DefaultDomains  *Domains `json:"default_domains,omitempty"`
}

// Domains represents custom domain configuration
type Domains struct {
	ReplicatedApp      string `json:"replicated_app,omitempty"`
	ProxyRegistry      string `json:"proxy_registry,omitempty"`
	ReplicatedRegistry string `json:"replicated_registry,omitempty"`
}

// getDomainsFromChannelReleases attempts to get domains from channel releases
func (c *APIClient) getDomainsFromChannelReleases() ([]string, error) {
	// First, get the list of channels
	channelsURL := fmt.Sprintf("%s/v3/app/%s/channels", c.apiOrigin, c.appID)

	if c.verbose {
		fmt.Fprintf(os.Stderr, "Fetching channels from: %s\n", channelsURL)
	}

	req, err := http.NewRequest("GET", channelsURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", c.apiToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get channels")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var channelsResp struct {
		Channels []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"channels"`
	}

	if err := json.Unmarshal(body, &channelsResp); err != nil {
		return nil, err
	}

	// Collect unique domains from all channels
	domainSet := make(map[string]bool)

	for _, channel := range channelsResp.Channels {
		// Get releases for this channel
		releasesURL := fmt.Sprintf("%s/v3/app/%s/channel/%s/releases", c.apiOrigin, c.appID, channel.ID)

		req, err := http.NewRequest("GET", releasesURL, nil)
		if err != nil {
			continue
		}

		req.Header.Set("Authorization", c.apiToken)
		req.Header.Set("Accept", "application/json")

		resp, err := c.client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		var releasesResp ChannelReleasesResponse
		if err := json.Unmarshal(body, &releasesResp); err != nil {
			continue
		}

		// Extract domains from releases
		for _, release := range releasesResp.ChannelReleases {
			if release.DefaultDomains != nil {
				if release.DefaultDomains.ReplicatedApp != "" {
					domainSet[release.DefaultDomains.ReplicatedApp] = true
				}
				if release.DefaultDomains.ProxyRegistry != "" {
					domainSet[release.DefaultDomains.ProxyRegistry] = true
				}
				if release.DefaultDomains.ReplicatedRegistry != "" {
					domainSet[release.DefaultDomains.ReplicatedRegistry] = true
				}
			}
		}
	}

	// Convert set to slice
	var domains []string
	for domain := range domainSet {
		domains = append(domains, domain)
	}

	return domains, nil
}

// getDomainsFromApp attempts to get domains from app configuration
func (c *APIClient) getDomainsFromApp() ([]string, error) {
	url := fmt.Sprintf("%s/v3/app/%s", c.apiOrigin, c.appID)

	if c.verbose {
		fmt.Fprintf(os.Stderr, "Fetching app details from: %s\n", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", c.apiToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request for app %q: %w", c.appID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d for app %q: %s", resp.StatusCode, c.appID, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse app response looking for custom domains
	var appResp struct {
		App struct {
			CustomDomains *Domains `json:"custom_domains,omitempty"`
			Domains       *Domains `json:"domains,omitempty"`
		} `json:"app"`
	}

	if err := json.Unmarshal(body, &appResp); err != nil {
		return nil, fmt.Errorf("failed to parse app response: %w", err)
	}

	domainSet := make(map[string]bool)

	// Check both possible fields for domains
	if appResp.App.CustomDomains != nil {
		if appResp.App.CustomDomains.ReplicatedApp != "" {
			domainSet[appResp.App.CustomDomains.ReplicatedApp] = true
		}
		if appResp.App.CustomDomains.ProxyRegistry != "" {
			domainSet[appResp.App.CustomDomains.ProxyRegistry] = true
		}
		if appResp.App.CustomDomains.ReplicatedRegistry != "" {
			domainSet[appResp.App.CustomDomains.ReplicatedRegistry] = true
		}
	}

	if appResp.App.Domains != nil {
		if appResp.App.Domains.ReplicatedApp != "" {
			domainSet[appResp.App.Domains.ReplicatedApp] = true
		}
		if appResp.App.Domains.ProxyRegistry != "" {
			domainSet[appResp.App.Domains.ProxyRegistry] = true
		}
		if appResp.App.Domains.ReplicatedRegistry != "" {
			domainSet[appResp.App.Domains.ReplicatedRegistry] = true
		}
	}

	// Convert set to slice
	var domains []string
	for domain := range domainSet {
		domains = append(domains, domain)
	}

	return domains, nil
}
