// Package release contains function to help finding things out about a given
// embedded cluster release. It is being kept here so if we decide to manage
// releases in a different way, we can easily change it.
package release

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	baseURL = "https://github.com/replicatedhq/embedded-cluster"
	pathFmt = "releases/download/v%s/metadata.json"
)

// Versions holds a list of add-on versions.
type Versions struct {
	AdminConsole            string
	EmbeddedClusterOperator string
	Installer               string
	Kubernetes              string
	OpenEBS                 string
}

// Meta represents the components of a given embedded cluster release. This
// is read directly from GitHub releases page.
type Meta struct {
	Versions     Versions
	K0sSHA       string
	K0sBinaryURL string
}

// MetadataFor reads metadata for a given release. Goes to GitHub releases page
// and reads versions.json file.
func MetadataFor(ctx context.Context, version string) (*Meta, error) {
	path := fmt.Sprintf(pathFmt, version)
	url := fmt.Sprintf("%s/%s", baseURL, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get bundle: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get bundle: %s", resp.Status)
	}
	var meta Meta
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("failed to decode bundle: %w", err)
	}
	return &meta, nil
}
