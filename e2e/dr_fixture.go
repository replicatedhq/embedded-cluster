package e2e

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster/cmx"
)

const drFixtureSchema = "embedded-cluster-dr-fixture/v1"

type drFixtureManifest struct {
	Schema        string `json:"schema"`
	CreatedAt     string `json:"createdAt"`
	ECVersion     string `json:"ecVersion"`
	K0sVersion    string `json:"k0sVersion"`
	Application   string `json:"applicationVersion"`
	S3Region      string `json:"s3Region"`
	S3Bucket      string `json:"s3Bucket"`
	S3Prefix      string `json:"s3Prefix"`
	S3AccessKey   string `json:"s3AccessKey"`
	S3SecretKey   string `json:"s3SecretKey"`
	Payload       string `json:"payload"`
	PayloadSHA256 string `json:"payloadSHA256"`
}

func exportDRFixture(tc *cmx.Cluster, minio *cmx.Minio, prefix, output string) error {
	if output == "" {
		return nil
	}

	const remoteFixture = "/tmp/embedded-cluster-dr-fixture.tar.gz"
	stdout, stderr, err := tc.RunCommandOnNode(0, []string{
		"tar", "--sort=name", "--mtime=@0", "--owner=0", "--group=0", "--numeric-owner",
		"-czf", remoteFixture, "-C", "/minio", "data",
	})
	if err != nil {
		return fmt.Errorf("archive MinIO fixture: %w: %s: %s", err, stdout, stderr)
	}

	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("create fixture output directory: %w", err)
	}
	if err := tc.CopyFileFromNode(0, remoteFixture, output); err != nil {
		return fmt.Errorf("copy fixture from CMX node: %w", err)
	}

	digest, err := fileSHA256(output)
	if err != nil {
		return err
	}
	applicationVersion := os.Getenv("E2E_DR_FIXTURE_APP_VERSION")
	if applicationVersion == "" {
		applicationVersion = fmt.Sprintf("appver-%s", os.Getenv("SHORT_SHA"))
	}
	manifest := drFixtureManifest{
		Schema:        drFixtureSchema,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		ECVersion:     os.Getenv("E2E_DR_FIXTURE_EC_VERSION"),
		K0sVersion:    k8sVersion(),
		Application:   applicationVersion,
		S3Region:      minio.Region,
		S3Bucket:      minio.DefaultBucket,
		S3Prefix:      prefix,
		S3AccessKey:   minio.AccessKey,
		S3SecretKey:   minio.SecretKey,
		Payload:       filepath.Base(output),
		PayloadSHA256: digest,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal fixture manifest: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(output+".manifest.json", data, 0o600); err != nil {
		return fmt.Errorf("write fixture manifest: %w", err)
	}
	return nil
}

func verifyDRFixture(payloadPath, manifestPath string) (*drFixtureManifest, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read fixture manifest: %w", err)
	}
	var manifest drFixtureManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("decode fixture manifest: %w", err)
	}
	if manifest.Schema != drFixtureSchema || manifest.ECVersion == "" || manifest.K0sVersion == "" ||
		manifest.Application == "" || manifest.S3Region == "" || manifest.S3Bucket == "" ||
		manifest.S3Prefix == "" || manifest.S3AccessKey == "" || manifest.S3SecretKey == "" {
		return nil, fmt.Errorf("fixture manifest is incomplete or has unsupported schema %q", manifest.Schema)
	}
	if manifest.Payload != filepath.Base(payloadPath) {
		return nil, fmt.Errorf("fixture manifest payload %q does not match %q", manifest.Payload, filepath.Base(payloadPath))
	}
	digest, err := fileSHA256(payloadPath)
	if err != nil {
		return nil, err
	}
	if digest != manifest.PayloadSHA256 {
		return nil, fmt.Errorf("fixture payload digest mismatch: got %s, want %s", digest, manifest.PayloadSHA256)
	}
	if err := verifyDRFixtureArchive(payloadPath); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func verifyDRFixtureArchive(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open fixture archive: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("open fixture gzip stream: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	files := 0
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read fixture archive: %w", err)
		}
		clean := filepath.ToSlash(filepath.Clean(header.Name))
		if clean != "data" && !strings.HasPrefix(clean, "data/") {
			return fmt.Errorf("fixture archive entry escapes data directory: %q", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
		case tar.TypeReg, tar.TypeRegA:
			files++
		default:
			return fmt.Errorf("fixture archive contains unsupported entry type for %q", header.Name)
		}
	}
	if files == 0 {
		return fmt.Errorf("fixture archive contains no MinIO objects")
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
