package e2e

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

// downloadAirgapBundle downloads the airgap bundle for the given version to the destination path.
// It retries the download up to 5 times if the bundle is less than 1GB.
// It cannot call t.Fatalf as it is used in a goroutine.
func downloadAirgapBundle(t *testing.T, versionLabel string, destPath string, licenseID string) error {
	for i := 0; i < 5; i++ {
		size, err := maybeDownloadAirgapBundle(t, versionLabel, destPath, licenseID)
		if err != nil {
			return fmt.Errorf("failed to download airgap bundle for version %s: %w", versionLabel, err)
		}
		if size > 1024*1024*1024 { // more than a GB
			t.Logf("downloaded airgap bundle to %s (%d bytes)", destPath, size)
			return nil
		}
		t.Logf("downloaded airgap bundle to %s (%d bytes), retrying as it is less than 1GB", destPath, size)
		err = os.RemoveAll(destPath)
		if err != nil {
			return fmt.Errorf("failed to remove airgap bundle at %s: %w", destPath, err)
		}
		time.Sleep(2 * time.Minute)
	}
	return fmt.Errorf("failed to download airgap bundle for version %s after 5 attempts", versionLabel)
}

func maybeDownloadAirgapBundle(t *testing.T, versionLabel string, destPath string, licenseID string) (int64, error) {
	// download airgap bundle
	airgapURL := fmt.Sprintf("https://staging.replicated.app/embedded/embedded-cluster-smoke-test-staging-app/ci-airgap/%s?airgap=true", versionLabel)

	req, err := http.NewRequest("GET", airgapURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", licenseID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code %s", resp.Status)
	}

	// pipe response to a temporary file
	airgapBundlePath := destPath
	f, err := os.Create(airgapBundlePath)
	if err != nil {
		return 0, fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer f.Close()
	size, err := f.ReadFrom(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to write response to temporary file: %w", err)
	}

	return size, nil
}
