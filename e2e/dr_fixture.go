package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster/cmx"
	"github.com/replicatedhq/embedded-cluster/pkg/drfixture"
)

// Aliases keep the E2E fixture tests close to the producer while the reusable
// implementation lives in pkg/drfixture for workflow and consumer commands.
const drFixtureSchema = drfixture.Schema

type drFixtureManifest = drfixture.Manifest

func verifyDRFixture(payloadPath, manifestPath string) (*drFixtureManifest, error) {
	return drfixture.Verify(payloadPath, manifestPath)
}

func fileSHA256(path string) (string, error) {
	return drfixture.FileSHA256(path)
}

func exportDRFixture(tc *cmx.Cluster, minio *cmx.Minio, prefix, output string) error {
	if output == "" {
		return nil
	}

	const remoteFixture = "/minio/embedded-cluster-dr-fixture.tar.gz"
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

	digest, err := drfixture.FileSHA256(output)
	if err != nil {
		return err
	}
	applicationVersion := os.Getenv("E2E_DR_FIXTURE_APP_VERSION")
	if applicationVersion == "" {
		applicationVersion = fmt.Sprintf("appver-%s", os.Getenv("SHORT_SHA"))
	}
	manifest := drfixture.Manifest{
		Schema:        drfixture.Schema,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		ECVersion:     os.Getenv("E2E_DR_FIXTURE_EC_VERSION"),
		K0sVersion:    k8sVersion(),
		Application:   applicationVersion,
		BundleSHA256:  os.Getenv("E2E_DR_FIXTURE_BUNDLE_SHA256"),
		S3Region:      minio.Region,
		S3Bucket:      minio.DefaultBucket,
		S3Prefix:      prefix,
		S3AccessKey:   minio.AccessKey,
		S3SecretKey:   minio.SecretKey,
		Payload:       filepath.Base(output),
		PayloadSHA256: digest,
	}
	return drfixture.WriteManifest(output+".manifest.json", manifest)
}

func restoreExportedDRFixture(tc *cmx.Cluster) error {
	const remoteFixture = "/minio/embedded-cluster-dr-fixture.tar.gz"
	stdout, stderr, err := tc.RunCommandOnNode(0, []string{
		"sh", "-eu", "-c",
		"rm -rf /minio/data && tar -xzf \"$1\" -C /minio", "--", remoteFixture,
	})
	if err != nil {
		return fmt.Errorf("replace MinIO data with exported fixture: %w: %s: %s", err, stdout, stderr)
	}
	return nil
}
