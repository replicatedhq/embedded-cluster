package e2e

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyDRFixture(t *testing.T) {
	dir := t.TempDir()
	payload := filepath.Join(dir, "fixture.tar.gz")
	writeFixtureArchive(t, payload, "data/e2e/prefix/object", []byte("backup"), tar.TypeReg)
	digest, err := fileSHA256(payload)
	if err != nil {
		t.Fatal(err)
	}
	manifest := validDRFixtureManifest(filepath.Base(payload), digest)
	manifestPath := payload + ".manifest.json"
	writeFixtureManifest(t, manifestPath, manifest)

	got, err := verifyDRFixture(payload, manifestPath)
	if err != nil {
		t.Fatalf("verify fixture: %v", err)
	}
	if got.PayloadSHA256 != digest {
		t.Fatalf("unexpected digest %q", got.PayloadSHA256)
	}
}

func TestVerifyDRFixtureRejectsDigestMismatch(t *testing.T) {
	dir := t.TempDir()
	payload := filepath.Join(dir, "fixture.tar.gz")
	writeFixtureArchive(t, payload, "data/e2e/prefix/object", []byte("backup"), tar.TypeReg)
	manifest := validDRFixtureManifest(filepath.Base(payload), "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	manifestPath := payload + ".manifest.json"
	writeFixtureManifest(t, manifestPath, manifest)

	if _, err := verifyDRFixture(payload, manifestPath); err == nil {
		t.Fatal("expected digest mismatch")
	}
}

func TestVerifyDRFixtureRejectsUnsafeArchiveEntry(t *testing.T) {
	dir := t.TempDir()
	payload := filepath.Join(dir, "fixture.tar.gz")
	writeFixtureArchive(t, payload, "../secret", []byte("backup"), tar.TypeReg)
	digest, err := fileSHA256(payload)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := payload + ".manifest.json"
	writeFixtureManifest(t, manifestPath, validDRFixtureManifest(filepath.Base(payload), digest))

	if _, err := verifyDRFixture(payload, manifestPath); err == nil {
		t.Fatal("expected unsafe archive entry error")
	}
}

func TestVerifyDRFixtureRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	payload := filepath.Join(dir, "fixture.tar.gz")
	writeFixtureArchive(t, payload, "data/link", nil, tar.TypeSymlink)
	digest, err := fileSHA256(payload)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := payload + ".manifest.json"
	writeFixtureManifest(t, manifestPath, validDRFixtureManifest(filepath.Base(payload), digest))

	if _, err := verifyDRFixture(payload, manifestPath); err == nil {
		t.Fatal("expected unsupported archive entry error")
	}
}

func validDRFixtureManifest(payload, digest string) drFixtureManifest {
	return drFixtureManifest{
		Schema:        drFixtureSchema,
		CreatedAt:     "2026-09-10T00:00:00Z",
		ECVersion:     "2.19.8+k8s-1.36",
		K0sVersion:    "v1.36.2+k0s.0",
		Application:   "appver-airgap-e2e-previous-stable",
		S3Region:      "us-east-1",
		S3Bucket:      "e2e",
		S3Prefix:      "fixture",
		S3AccessKey:   "fixture-access",
		S3SecretKey:   "fixture-secret",
		Payload:       payload,
		PayloadSHA256: digest,
	}
}

func writeFixtureManifest(t *testing.T, path string, manifest drFixtureManifest) {
	t.Helper()
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeFixtureArchive(t *testing.T, path, name string, contents []byte, typeflag byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	header := &tar.Header{Name: name, Mode: 0o600, Size: int64(len(contents)), Typeflag: typeflag}
	if typeflag == tar.TypeSymlink {
		header.Linkname = "../secret"
		header.Size = 0
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if len(contents) > 0 {
		if _, err := tw.Write(contents); err != nil {
			t.Fatal(err)
		}
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
