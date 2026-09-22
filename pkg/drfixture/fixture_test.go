package drfixture

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildAddsPayloadIdentityAndRequiresProvenance(t *testing.T) {
	payload := filepath.Join(t.TempDir(), "fixture.tar.gz")
	writeTarGzip(t, payload, "data/e2e/prefix/object", []byte("fixture contents"))
	manifest := Manifest{
		CreatedAt: time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		ECVersion: "v2.19.8+k8s-1.35", K0sVersion: "v1.35.6",
		Application:  "appver-airgap-e2e-previous-stable",
		BundleSHA256: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		S3Region:     "us-east-1", S3Bucket: "e2e", S3Prefix: "fixture",
		S3AccessKey: "fixture-access", S3SecretKey: "fixture-secret",
		ECCommit:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		KOTSCommit:    "cccccccccccccccccccccccccccccccccccccccc",
		VeleroVersion: "v1.18.2",
		Generation:    "gh workflow run e2e-dr-fixture-candidate.yaml",
		KOTSDigests:   []string{"sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"},
	}
	built, err := Build(payload, manifest)
	if err != nil {
		t.Fatalf("build fixture manifest: %v", err)
	}
	if built.Schema != Schema || built.Payload != filepath.Base(payload) || built.PayloadSHA256 == "" {
		t.Fatalf("unexpected derived manifest fields: %#v", built)
	}
	if err := WriteManifest(payload+".manifest.json", *built); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(payload, payload+".manifest.json"); err != nil {
		t.Fatalf("verify built manifest: %v", err)
	}
	manifest.ECCommit = ""
	if _, err := Build(payload, manifest); err == nil {
		t.Fatal("expected missing provenance to be rejected")
	}
}

func TestRejectValuesFindsValueAcrossReadBoundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload")
	data := append(make([]byte, 64*1024-3), []byte("fixture-secret")...)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RejectValues(path, map[string]string{"test credential": "fixture-secret"}); err == nil {
		t.Fatal("expected forbidden value error")
	}
}

func TestRejectValuesFindsValueInNestedTarGzip(t *testing.T) {
	dir := t.TempDir()
	inner := filepath.Join(dir, "inner.tar.gz")
	writeTarGzip(t, inner, "backup/credentials", []byte("prefix-fixture-secret-suffix"))
	innerData, err := os.ReadFile(inner)
	if err != nil {
		t.Fatal(err)
	}
	outer := filepath.Join(dir, "outer.tar.gz")
	writeTarGzip(t, outer, "data/bucket/backup.tar.gz", innerData)
	if err := RejectValues(outer, map[string]string{"test credential": "fixture-secret"}); err == nil {
		t.Fatal("expected forbidden value error")
	}
}

func TestRejectValuesAcceptsCleanNestedArchive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload.tar.gz")
	writeTarGzip(t, path, "data/bucket/backup", []byte("fixture contents"))
	if err := RejectValues(path, map[string]string{"test credential": "absent-secret"}); err != nil {
		t.Fatalf("inspect clean fixture: %v", err)
	}
}

func writeTarGzip(t *testing.T, path, name string, contents []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(contents)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
