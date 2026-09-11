package cmx

import (
	"fmt"
	"os"
	"os/exec"
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

	minioBinary, err := localToolPath("E2E_MINIO_BINARY", "minio")
	if err != nil {
		return nil, err
	}
	if err := c.CopyFileToNode(node, minioBinary, "/minio/bin/minio"); err != nil {
		return nil, fmt.Errorf("copy minio: %w", err)
	}

	// Make binary executable
	if stdout, stderr, err := c.RunCommandOnNode(node, []string{"chmod", "+x", "/minio/bin/minio"}); err != nil {
		return nil, fmt.Errorf("chmod minio: %v: %s: %s", err, stdout, stderr)
	}

	mcBinary, err := localToolPath("E2E_MC_BINARY", "mc")
	if err != nil {
		return nil, err
	}
	if err := c.CopyFileToNode(node, mcBinary, "/minio/bin/mc"); err != nil {
		return nil, fmt.Errorf("copy mc: %w", err)
	}

	// Make binary executable
	if stdout, stderr, err := c.RunCommandOnNode(node, []string{"chmod", "+x", "/minio/bin/mc"}); err != nil {
		return nil, fmt.Errorf("chmod mc: %v: %s: %s", err, stdout, stderr)
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

func localToolPath(envName, binaryName string) (string, error) {
	path := os.Getenv(envName)
	if path == "" {
		var err error
		path, err = exec.LookPath(binaryName)
		if err != nil {
			return "", fmt.Errorf("%s is not set and %s is not installed locally", envName, binaryName)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", binaryName, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s path %q is not a regular file", binaryName, path)
	}
	return path, nil
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

// StopMinio stops the fixture server so its object tree can be archived
// consistently. StartMinio can start the same server and data again later.
func (c *Cluster) StopMinio(node int) error {
	stdout, stderr, err := c.RunCommandOnNode(node, []string{"pkill", "-x", "minio"})
	if err != nil {
		return fmt.Errorf("stop minio: %w: %s: %s", err, stdout, stderr)
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, _, err := c.RunCommandOnNode(node, []string{"pgrep", "-x", "minio"}); err != nil {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("timeout waiting for minio to stop")
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
