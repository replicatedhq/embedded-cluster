package e2e

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalAirgapBundle(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "snapshot", "v1.tgz")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte("bundle")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if err := os.WriteFile(path+".sha256", []byte(fmt.Sprintf("%x  v1.tgz\n", sum)), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf(`{"schema":"v1","application":{"path":"app.airgap","sha256":"sha256:%x"},"ecFiles":{"embedded-cluster/embedded-cluster-amd64":"sha256:%x"}}`, sum, sum)
	if err := os.WriteFile(path+".manifest.json", []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("E2E_AIRGAP_BUNDLE_DIR", root)

	got, enabled, err := localAirgapBundle("v1", AirgapInstallBundlePath, AirgapSnapshotLicenseID)
	if err != nil {
		t.Fatal(err)
	}
	if !enabled || got != path {
		t.Fatalf("got path %q, enabled %v", got, enabled)
	}
}

func TestLocalAirgapBundleRejectsNonCanonicalDigest(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "standard", "v1.tgz")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".manifest.json", []byte(`{"schema":"v1","application":{"path":"app.airgap","sha256":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"ecFiles":{"embedded-cluster/version-metadata.json":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("E2E_AIRGAP_BUNDLE_DIR", root)

	if _, enabled, err := localAirgapBundle("v1", AirgapInstallBundlePath, AirgapLicenseID); !enabled || err == nil {
		t.Fatalf("expected non-canonical component digest to fail")
	}
}

func TestLocalAirgapBundleFailsClosed(t *testing.T) {
	t.Setenv("E2E_AIRGAP_BUNDLE_DIR", t.TempDir())
	if _, enabled, err := localAirgapBundle("missing", AirgapInstallBundlePath, AirgapLicenseID); !enabled || err == nil {
		t.Fatalf("expected enabled local bundle lookup to fail")
	}
}

func TestLocalAirgapBundleNameUsesDistinctAppOnlyUpgrade(t *testing.T) {
	t.Setenv("SHORT_SHA", "airgap-e2e")
	version := "appver-airgap-e2e"
	if got := localAirgapBundleName(version, AirgapInstallBundlePath); got != version {
		t.Fatalf("install bundle name = %q", got)
	}
	if got := localAirgapBundleName(version, AirgapUpgradeBundlePath); got != version+"-app-only-upgrade" {
		t.Fatalf("app-only upgrade bundle name = %q", got)
	}
}
