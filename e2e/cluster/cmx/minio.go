package cmx

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Minio contains the endpoint and credentials for Minio
type Minio struct {
	Endpoint      string
	Region        string
	AccessKey     string
	SecretKey     string
	DefaultBucket string
}

// DeployMinio deploys Minio to the specified node and returns the endpoint and credentials
func (c *Cluster) DeployMinio(node int) (*Minio, error) {
	// Create directories
	stdout, stderr, err := c.RunCommandOnNode(node, []string{"mkdir", "-p", "/minio/data", "/minio/bin"})
	if err != nil {
		return nil, fmt.Errorf("create minio directories: %v: %s: %s", err, stdout, stderr)
	}

	// dl.min.io no longer serves community binaries. Pin the GitHub release
	// assets and their SHA-256 checksums for reproducible test fixtures.
	binaries := []struct {
		name    string
		version string
		sha256  string
	}{
		{
			name:    "minio",
			version: "RELEASE.2025-09-07T16-13-09Z",
			sha256:  "7c5bd8512c6e966455b1d198209358b2d191c77a83ab377c4073281065fb855f",
		},
		{
			name:    "mc",
			version: "RELEASE.2025-08-13T08-35-41Z",
			sha256:  "01f866e9c5f9b87c2b09116fa5d7c06695b106242d829a8bb32990c00312e891",
		},
	}
	for _, binary := range binaries {
		url := fmt.Sprintf("https://github.com/minio/%s/releases/download/%s/%s.linux-amd64.%s", binary.name, binary.version, binary.name, binary.version)
		path := "/minio/bin/" + binary.name
		if stdout, stderr, err := c.RunCommandOnNode(node, []string{"curl", "-fSL", url, "-o", path}); err != nil {
			return nil, fmt.Errorf("download %s: %v: %s: %s", binary.name, err, stdout, stderr)
		}

		stdout, stderr, err := c.RunCommandOnNode(node, []string{"sha256sum", path})
		if err != nil {
			return nil, fmt.Errorf("checksum %s: %v: %s: %s", binary.name, err, stdout, stderr)
		}
		if fields := strings.Fields(stdout); len(fields) == 0 || fields[0] != binary.sha256 {
			return nil, fmt.Errorf("checksum %s: expected %s, got %q", binary.name, binary.sha256, stdout)
		}

		if stdout, stderr, err := c.RunCommandOnNode(node, []string{"chmod", "+x", path}); err != nil {
			return nil, fmt.Errorf("chmod %s: %v: %s: %s", binary.name, err, stdout, stderr)
		}
	}

	// Generate credentials
	accessKey := uuid.New().String()
	secretKey := uuid.New().String()

	// Minio details
	minio := &Minio{
		Endpoint:      fmt.Sprintf("http://%s:9000", c.Nodes[node].privateIP),
		Region:        "us-east-1",
		AccessKey:     accessKey,
		SecretKey:     secretKey,
		DefaultBucket: "e2e",
	}

	// Start Minio
	if err := c.StartMinio(node, minio); err != nil {
		return nil, fmt.Errorf("start minio: %w", err)
	}

	// Configure mc with MinIO credentials
	configCmd := []string{
		"/minio/bin/mc", "alias", "set", "e2e-minio",
		minio.Endpoint,
		minio.AccessKey,
		minio.SecretKey,
	}
	if stdout, stderr, err := c.RunCommandOnNode(node, configCmd); err != nil {
		return nil, fmt.Errorf("configure mc: %v: %s: %s", err, stdout, stderr)
	}

	// Create default bucket
	if stdout, stderr, err := c.RunCommandOnNode(node, []string{"/minio/bin/mc", "mb", fmt.Sprintf("e2e-minio/%s", minio.DefaultBucket)}); err != nil {
		return nil, fmt.Errorf("create default bucket: %v: %s: %s", err, stdout, stderr)
	}

	return minio, nil
}

func (c *Cluster) StartMinio(node int, minio *Minio) error {
	go func() {
		envs := map[string]string{
			"MINIO_ACCESS_KEY": minio.AccessKey,
			"MINIO_SECRET_KEY": minio.SecretKey,
		}

		line := []string{"/minio/bin/minio", "server", "/minio/data", "--address", ":9000"}
		if stdout, stderr, err := c.RunCommandOnNode(node, line, envs); err != nil {
			c.t.Logf("minio server: %v: %s: %s", err, stdout, stderr)
		}
	}()

	if err := c.waitForMinio(node, minio); err != nil {
		return fmt.Errorf("wait for minio: %w", err)
	}

	return nil
}

func (c *Cluster) waitForMinio(node int, minio *Minio) error {
	startTime := time.Now()

	for {
		err := c.checkMinioReady(node, minio)
		if err == nil {
			return nil
		}

		if time.Since(startTime) > 1*time.Minute {
			return fmt.Errorf("timeout waiting for minio to be ready: %w", err)
		}

		time.Sleep(2 * time.Second)
	}
}

func (c *Cluster) checkMinioReady(node int, minio *Minio) error {
	stdout, stderr, err := c.RunCommandOnNode(node, []string{"curl", minio.Endpoint})
	if err != nil {
		return fmt.Errorf("do request: %w: %s: %s", err, stdout, stderr)
	}

	if !strings.Contains(stdout, "AccessDenied") {
		return fmt.Errorf("unexpected response: %s: %s", stdout, stderr)
	}

	return nil
}
