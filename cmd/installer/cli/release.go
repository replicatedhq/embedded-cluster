package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/replicatedhq/kotskinds/pkg/licensewrapper"
)

type apiChannelRelease struct {
	ChannelID                string `json:"channelId"`
	ChannelSequence          int64  `json:"channelSequence"`
	ReleaseSequence          int64  `json:"releaseSequence"`
	VersionLabel             string `json:"versionLabel"`
	IsRequired               bool   `json:"isRequired"`
	SemVer                   string `json:"semver,omitempty"`
	CreatedAt                string `json:"createdAt"`
	ReleaseNotes             string `json:"releaseNotes"`
	ReplicatedRegistryDomain string `json:"replicatedRegistryDomain"`
	ReplicatedProxyDomain    string `json:"replicatedProxyDomain"`
}

func getCurrentAppChannelRelease(ctx context.Context, license *licensewrapper.LicenseWrapper, channelID string) (*apiChannelRelease, error) {
	if license == nil {
		return nil, fmt.Errorf("license is required")
	}

	query := url.Values{}
	query.Set("selectedChannelId", channelID)
	query.Set("channelSequence", "") // sending an empty string will return the latest channel release
	query.Set("isSemverSupported", "true")

	apiURL := replicatedAppURL()
	url := fmt.Sprintf("%s/release/%s/pending?%s", apiURL, license.GetAppSlug(), query.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	licenseID := license.GetLicenseID()
	auth := fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", licenseID, licenseID))))
	req.Header.Set("Authorization", auth)

	// This will use the proxy from the environment if set by the cli command.
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get pending app releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %s", resp.Status)
	}

	var releases struct {
		ChannelReleases []apiChannelRelease `json:"channelReleases"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode pending app releases: %w", err)
	}

	if len(releases.ChannelReleases) == 0 {
		return nil, errors.New("no app releases found")
	}

	return &releases.ChannelReleases[0], nil
}
