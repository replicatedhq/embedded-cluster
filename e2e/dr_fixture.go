package e2e

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	manifest := drFixtureManifest{
		Schema:        drFixtureSchema,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		ECVersion:     os.Getenv("E2E_DR_FIXTURE_EC_VERSION"),
		K0sVersion:    k8sVersion(),
		Application:   fmt.Sprintf("appver-%s", os.Getenv("SHORT_SHA")),
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
