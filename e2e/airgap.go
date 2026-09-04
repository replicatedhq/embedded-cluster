package e2e

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster"
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
	if source, ok, err := localAirgapBundle(versionLabel, licenseID); ok {
		if err != nil {
			return err
		}
		return copyLocalBundle(source, destPath)
	}
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
			err = os.RemoveAll(destPath)
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
		_ = os.RemoveAll(airgapBundlePath)
		return 0, fmt.Errorf("failed to write response to temporary file: %w", err)
	}

	return size, nil
}

func downloadAirgapBundleOnNode(t *testing.T, tc cluster.Cluster, node int, versionLabel string, destPath string, licenseID string) error {
	if source, ok, err := localAirgapBundle(versionLabel, licenseID); ok {
		if err != nil {
			return err
		}
		t.Logf("staging verified local airgap bundle %s on node %d", source, node)
		return copyLocalBundleToNode(tc, node, source, destPath)
	}
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

func copyLocalBundleToNode(tc cluster.Cluster, node int, source, dest string) error {
	// CMX connects as an unprivileged SSH user, so scp cannot write directly to
	// /assets. Upload the complete bundle to a writable location first, then use
	// the cluster's privileged command runner to move it into place atomically.
	temporary := filepath.Join("/tmp", filepath.Base(dest)+".partial")
	if err := tc.CopyFileToNode(node, source, temporary); err != nil {
		return err
	}
	stdout, stderr, err := tc.RunCommandOnNode(node, []string{
		"mkdir", "-p", filepath.Dir(dest), "&&", "sudo", "mv", temporary, dest,
	})
	if err != nil {
		_, _, _ = tc.RunCommandOnNode(node, []string{"rm", "-f", temporary})
		return fmt.Errorf("move local airgap bundle into place: %w: %s: %s", err, stdout, stderr)
	}
	return nil
}

func localAirgapBundle(versionLabel, licenseID string) (string, bool, error) {
	root := os.Getenv("E2E_AIRGAP_BUNDLE_DIR")
	if root == "" {
		return "", false, nil
	}
	variant := "standard"
	if licenseID == AirgapSnapshotLicenseID {
		variant = "snapshot"
	}
	path := fmt.Sprintf("%s/%s/%s.tgz", root, variant, versionLabel)
	manifestData, err := os.ReadFile(path + ".manifest.json")
	if err != nil {
		return "", true, fmt.Errorf("local airgap bundle manifest is required for %s: %w", path, err)
	}
	var manifest struct {
		Schema      string `json:"schema"`
		Application struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"application"`
		ECFiles map[string]string `json:"ecFiles"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil || manifest.Schema != "v1" || len(manifest.ECFiles) == 0 {
		return "", true, fmt.Errorf("invalid local airgap bundle manifest for %s", path)
	}
	if manifest.Application.Path == "" || !validSHA256Digest(manifest.Application.SHA256) {
		return "", true, fmt.Errorf("invalid application input in local airgap bundle manifest for %s", path)
	}
	for name, digest := range manifest.ECFiles {
		if !validSHA256Digest(digest) {
			return "", true, fmt.Errorf("invalid component digest for %s in %s", name, path)
		}
	}
	checksum, err := os.ReadFile(path + ".sha256")
	if err != nil {
		return "", true, fmt.Errorf("local airgap bundle checksum is required for %s: %w", path, err)
	}
	f, err := os.Open(path)
	if err != nil {
		return "", true, fmt.Errorf("local airgap bundle is required for %s: %w", versionLabel, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", true, err
	}
	want := strings.Fields(string(checksum))
	if len(want) == 0 || want[0] != hex.EncodeToString(h.Sum(nil)) {
		return "", true, fmt.Errorf("local airgap bundle checksum mismatch: %s", path)
	}
	return path, true, nil
}

func validSHA256Digest(value string) bool {
	encoded, ok := strings.CutPrefix(value, "sha256:")
	if !ok {
		return false
	}
	decoded, err := hex.DecodeString(encoded)
	return err == nil && len(decoded) == sha256.Size
}

func copyLocalBundle(source, dest string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
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
