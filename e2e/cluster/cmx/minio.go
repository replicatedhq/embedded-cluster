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

	// Install Go (only used for downloading minio and mc as the official mirrors get throttled in cmx)
	stdout, stderr, err = c.RunCommandOnNode(node, []string{"curl", "-L", "https://go.dev/dl/go1.24.2.linux-amd64.tar.gz", "|", "sudo", "tar", "-C", "/usr/local", "-xz"})
	if err != nil {
		return nil, fmt.Errorf("install go: %v: %s: %s", err, stdout, stderr)
	}

	// Download minio binary
	downloadEnvs := map[string]string{"GOBIN": "/minio/bin"}
	downloadCmd := []string{"/usr/local/go/bin/go", "install", "github.com/minio/minio@v0.0.0-20250507153712-6d18dba9a20d"}
	if stdout, stderr, err := c.RunCommandOnNode(node, downloadCmd, downloadEnvs); err != nil {
		return nil, fmt.Errorf("download minio: %v: %s: %s", err, stdout, stderr)
	}

	// Download mc binary
	downloadCmd = []string{"/usr/local/go/bin/go", "install", "github.com/minio/mc@v0.0.0-20250506164133-19d87ba47505"}
	if stdout, stderr, err := c.RunCommandOnNode(node, downloadCmd, downloadEnvs); err != nil {
		return nil, fmt.Errorf("download mc: %v: %s: %s", err, stdout, stderr)
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
