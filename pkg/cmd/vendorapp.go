package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
)

const (
	warnNewVersionPrompt = "A newer version %s is available. Run 'curl -f \"%s\" -H \"Authorization: %s\" -o %s-%s.tgz' to download it."
)

// maybePromptForAppUpdate warns the user if there are any pending app releases for the current
// channel. If prompt is not nil, it will prompt the user to continue installing the out-of-date
// release and return an error if the user chooses not to continue.
func maybePromptForAppUpdate(ctx context.Context, license *kotsv1beta1.License, prompt prompts.Prompt) error {
	channelRelease, err := release.GetChannelRelease()
	if err != nil {
		return fmt.Errorf("unable to get channel release: %w", err)
	} else if channelRelease == nil {
		// It is possible to install without embedding the release data. In this case, we cannot
		// check for app updates.
		return nil
	}

	if license == nil {
		return errors.New("license required")
	}

	logrus.Debugf("Checking for pending app releases")

	currentRelease, err := getCurrentAppChannelRelease(ctx, license, channelRelease.ChannelID)
	if err != nil {
		return fmt.Errorf("get current app channel release: %w", err)
	}

	if currentRelease.VersionLabel == channelRelease.VersionLabel {
		logrus.Debugf("Current app release is up-to-date")
		return nil
	}
	logrus.Debugf("Current app release is out-of-date")

	apiURL := metrics.BaseURL(license)
	releaseURL := fmt.Sprintf("%s/embedded/%s/%s", apiURL, channelRelease.AppSlug, channelRelease.ChannelSlug)
	logrus.Warnf(
		warnNewVersionPrompt,
		currentRelease.VersionLabel,
		releaseURL,
		license.Spec.LicenseID,
		channelRelease.AppSlug,
		channelRelease.ChannelSlug,
	)

	// if prompt is nil, we don't prompt the user and continue by default.
	// SKIP_APP_UPDATE_PROMPT is an escape hatch used by the CI to skip the prompt in case this
	// release becomes out of date.
	if prompt != nil && os.Getenv("SKIP_APP_UPDATE_PROMPT") != "true" {
		text := fmt.Sprintf("Do you want to continue installing %s?", channelRelease.VersionLabel)
		if !prompt.Confirm(text, false) {
			return ErrNothingElseToAdd
		}
	}
	return nil
}

type apiChannelRelease struct {
	ChannelSequence int    `json:"channelSequence"`
	ReleaseSequence int    `json:"releaseSequence"`
	VersionLabel    string `json:"versionLabel"`
	IsRequired      bool   `json:"isRequired"`
	CreatedAt       string `json:"createdAt"`
	ReleaseNotes    string `json:"releaseNotes"`
}

func getCurrentAppChannelRelease(ctx context.Context, license *kotsv1beta1.License, channelID string) (*apiChannelRelease, error) {
	query := url.Values{}
	query.Set("selectedChannelId", channelID)
	query.Set("channelSequence", "") // sending an empty string will return the latest channel release
	query.Set("isSemverSupported", "true")

	apiURL := metrics.BaseURL(license)
	url := fmt.Sprintf("%s/release/%s/pending?%s", apiURL, license.Spec.AppSlug, query.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	auth := fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", license.Spec.LicenseID, license.Spec.LicenseID))))
	req.Header.Set("Authorization", auth)

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
