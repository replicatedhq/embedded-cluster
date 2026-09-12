package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	applicationVersion := os.Getenv("E2E_DR_FIXTURE_APP_VERSION")
	if applicationVersion == "" {
		applicationVersion = fmt.Sprintf("appver-%s", os.Getenv("SHORT_SHA"))
	}
	manifest, err := drfixture.Build(output, drfixture.Manifest{
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		ECVersion:     os.Getenv("E2E_DR_FIXTURE_EC_VERSION"),
		ECCommit:      os.Getenv("E2E_DR_FIXTURE_EC_COMMIT"),
		K0sVersion:    k8sVersion(),
		KOTSCommit:    os.Getenv("E2E_DR_FIXTURE_KOTS_COMMIT"),
		VeleroVersion: os.Getenv("E2E_DR_FIXTURE_VELERO_VERSION"),
		Application:   applicationVersion,
		BundleSHA256:  os.Getenv("E2E_DR_FIXTURE_BUNDLE_SHA256"),
		Generation:    os.Getenv("E2E_DR_FIXTURE_GENERATION_COMMAND"),
		KOTSDigests:   strings.Fields(os.Getenv("E2E_DR_FIXTURE_KOTS_DIGESTS")),
		S3Region:      minio.Region,
		S3Bucket:      minio.DefaultBucket,
		S3Prefix:      prefix,
		S3AccessKey:   minio.AccessKey,
		S3SecretKey:   minio.SecretKey,
	})
	if err != nil {
		return err
	}
	return drfixture.WriteManifest(output+".manifest.json", *manifest)
}

func restoreExportedDRFixture(tc *cmx.Cluster) error {
	const remoteFixture = "/minio/embedded-cluster-dr-fixture.tar.gz"
	stdout, stderr, err := tc.RunCommandOnNode(0, []string{"rm", "-rf", "/minio/data"})
	if err != nil {
		return fmt.Errorf("remove original MinIO data: %w: %s: %s", err, stdout, stderr)
	}
	stdout, stderr, err = tc.RunCommandOnNode(0, []string{
		"tar", "-xzf", remoteFixture, "-C", "/minio",
	})
	if err != nil {
		return fmt.Errorf("extract exported MinIO data: %w: %s: %s", err, stdout, stderr)
	}
	return nil
}

func stageDRFixture(tc *cmx.Cluster, payloadPath string, manifest *drFixtureManifest) (*cmx.Minio, error) {
	if _, err := os.Stat(payloadPath); err != nil {
		return nil, fmt.Errorf("stat DR fixture payload: %w", err)
	}

	// DeployMinio installs the server and client binaries before the nodes are
	// isolated. Replace its empty object tree with the immutable fixture and
	// restart it using the credentials recorded with that tree.
	if _, err := tc.DeployMinio(0); err != nil {
		return nil, fmt.Errorf("install fixture MinIO: %w", err)
	}
	if err := tc.StopMinio(0); err != nil {
		return nil, err
	}
	const remoteFixture = "/tmp/embedded-cluster-dr-fixture.tar.gz"
	if err := tc.CopyFileToNode(0, payloadPath, remoteFixture); err != nil {
		return nil, fmt.Errorf("copy DR fixture to CMX node: %w", err)
	}
	stdout, stderr, err := tc.RunCommandOnNode(0, []string{
		"rm", "-rf", "/minio/data", "&&", "sudo", "tar", "-xzf", remoteFixture, "-C", "/minio",
	})
	if err != nil {
		return nil, fmt.Errorf("extract DR fixture: %w: %s: %s", err, stdout, stderr)
	}

	minio := &cmx.Minio{
		Endpoint:      fmt.Sprintf("http://%s:9000", tc.NodePrivateIP(0)),
		Region:        manifest.S3Region,
		AccessKey:     manifest.S3AccessKey,
		SecretKey:     manifest.S3SecretKey,
		DefaultBucket: manifest.S3Bucket,
	}
	if err := tc.StartMinio(0, minio); err != nil {
		return nil, fmt.Errorf("start fixture MinIO: %w", err)
	}
	return minio, nil
}
