package e2e

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
)

const (
	AirgapInstallBundlePath  = "/assets/ec-release.tgz"
	AirgapUpgradeBundlePath  = "/assets/ec-release-upgrade.tgz"
	AirgapUpgrade2BundlePath = "/assets/ec-release-upgrade2.tgz"
)

// downloadAirgapBundle downloads the airgap bundle for the given version to the destination path.
// It retries the download up to 20 times if the bundle is less than 1GB.
// It cannot call t.Fatalf as it is used in a goroutine.
func downloadAirgapBundle(t *testing.T, versionLabel string, destPath string, licenseID string) error {
	for i := 0; i < 20; i++ {
		size, err := maybeDownloadAirgapBundle(versionLabel, destPath, licenseID)
		if err != nil {
			// when we deploy the api to staging it interrupts the download
			t.Logf("failed to download airgap bundle for version %s with error %q, retrying", versionLabel, err)
		} else {
			if size > 1024*1024*1024 { // more than a GB
				t.Logf("downloaded airgap bundle to %s (%d bytes)", destPath, size)
				return nil
			}
			t.Logf("downloaded airgap bundle to %s (%d bytes), retrying as it is less than 1GB", destPath, size)
			err = helpers.RemoveAll(destPath)
			if err != nil {
				return fmt.Errorf("failed to remove airgap bundle at %s: %w", destPath, err)
			}
		}
		time.Sleep(1 * time.Minute)
	}
	return fmt.Errorf("failed to download airgap bundle for version %s after 20 attempts", versionLabel)
}

func maybeDownloadAirgapBundle(versionLabel string, destPath string, licenseID string) (int64, error) {
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
		_ = f.Close()
		_ = helpers.RemoveAll(airgapBundlePath)
		return 0, fmt.Errorf("failed to write response to temporary file: %w", err)
	}

	return size, nil
}

func downloadAirgapBundleOnNode(t *testing.T, tc cluster.Cluster, node int, versionLabel string, destPath string, licenseID string) error {
	for range 20 {
		start := time.Now()
		size, err := maybeDownloadAirgapBundleOnNode(tc, node, versionLabel, destPath, licenseID)
		if err != nil {
			// when we deploy the api to staging it interrupts the download
			t.Logf("failed to download airgap bundle for version %s on node %d with error %q, retrying", versionLabel, node, err)
		} else {
			if size > 1 { // more than a GB
				t.Logf("downloaded airgap bundle on node %d to %s (%.1f GB) in %s", node, destPath, size, time.Since(start))
				return nil
			}
			t.Logf("downloaded airgap bundle on node %d to %s (%.1f GB), retrying as it is less than 1GB", node, destPath, size)
		}
		time.Sleep(1 * time.Minute)
	}
	return fmt.Errorf("failed to download airgap bundle for version %s on node %d after 20 attempts", versionLabel, node)
}

func maybeDownloadAirgapBundleOnNode(tc cluster.Cluster, node int, versionLabel string, destPath string, licenseID string) (float64, error) {
	// download airgap bundle
	airgapURL := fmt.Sprintf("https://staging.replicated.app/embedded/embedded-cluster-smoke-test-staging-app/ci-airgap/%s?airgap=true", versionLabel)

	stdout, stderr, err := tc.RunCommandOnNode(node, []string{"curl", "-f", "-H", fmt.Sprintf("'Authorization: %s'", licenseID), "-L", airgapURL, "-o", destPath})
	if err != nil {
		return 0, fmt.Errorf("failed to download airgap bundle: %v: %s: %s", err, stdout, stderr)
	}

	// get the size of the file on the node
	stdout, stderr, err = tc.RunCommandOnNode(node, []string{"du", "-h", destPath, "|", "awk", "'{print $1}'"})
	if err != nil {
		return 0, fmt.Errorf("failed to check file size: %v: %s: %s", err, stdout, stderr)
	}

	sizeStr := strings.TrimSpace(stdout)

	// match only if the size is in gigabytes
	re := regexp.MustCompile(`(?i)^([\d.]+)G$`)
	matches := re.FindStringSubmatch(sizeStr)
	if matches == nil {
		return 0, fmt.Errorf("file size is not in gigabytes: %s", sizeStr)
	}

	size, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse numeric value: %w", err)
	}
	return size, nil
}
