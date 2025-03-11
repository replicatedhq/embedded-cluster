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

	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
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

func getCurrentAppChannelRelease(ctx context.Context, license *kotsv1beta1.License, channelID string) (*apiChannelRelease, error) {
	query := url.Values{}
	query.Set("selectedChannelId", channelID)
	query.Set("channelSequence", "") // sending an empty string will return the latest channel release
	query.Set("isSemverSupported", "true")

	apiURL := fmt.Sprintf("https://%s", runtimeconfig.ReplicatedAppDomain(license))
	url := fmt.Sprintf("%s/release/%s/pending?%s", apiURL, license.Spec.AppSlug, query.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	auth := fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", license.Spec.LicenseID, license.Spec.LicenseID))))
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
