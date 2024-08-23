package docker

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type Docker struct {
	client string
	t      *testing.T
}

func NewCLI(t *testing.T) *Docker {
	client, err := exec.LookPath("docker")
	if err != nil {
		t.Fatalf("failed to find docker in path: %v", err)
	}
	return &Docker{
		client: client,
		t:      t,
	}
}

type Container struct {
	Image   string
	Volumes []string

	id string
	t  *testing.T
}

func NewContainer(t *testing.T) *Container {
	return &Container{
		id: generateID(),
		t:  t,
	}
}

func (c *Container) WithImage(image string) *Container {
	c.Image = image
	return c
}

func (c *Container) WithECBinary() *Container {
	path, err := filepath.Abs("../output/bin/embedded-cluster")
	if err != nil {
		c.t.Fatalf("failed to get absolute path to embedded-cluster binary: %v", err)
	}
	_, err = os.Stat(path)
	if err != nil {
		c.t.Fatalf("failed to find embedded-cluster binary: %v", err)
	}
	err = os.Chmod(path, 0755)
	if err != nil {
		c.t.Fatalf("failed to chmod embedded-cluster binary: %v", err)
	}
	return c.WithVolume(fmt.Sprintf("%s:%s", path, c.GetECBinaryPath()))
}

func (c *Container) GetECBinaryPath() string {
	return "/ec/bin/embedded-cluster"
}

func (c *Container) WithLicense(path string) *Container {
	path, err := filepath.Abs(path)
	if err != nil {
		c.t.Fatalf("failed to get absolute path to license file: %v", err)
	}
	_, err = os.Stat(path)
	if err != nil {
		c.t.Fatalf("failed to find embedded-cluster binary: %v", err)
	}
	return c.WithVolume(fmt.Sprintf("%s:%s", path, c.GetLicensePath()))
}

func (c *Container) GetLicensePath() string {
	return "/ec/license.yaml"
}

func (c *Container) WithVolume(volume string) *Container {
	c.Volumes = append(c.Volumes, volume)
	return c
}

func (c *Container) Start(cli *Docker) {
	execCmd := exec.Command(
		cli.client, "run", "--rm", "-d", "-w", "/ec", "--platform=linux/amd64", "--privileged",
		"--name", c.id,
	)
	for _, volume := range c.Volumes {
		execCmd.Args = append(execCmd.Args, "-v", volume)
	}
	execCmd.Args = append(execCmd.Args, c.Image)
	execCmd.Args = append(execCmd.Args, "sh", "-c", "while true; do sleep 1; done")
	c.t.Logf("starting container: docker %s", strings.Join(execCmd.Args, " "))
	err := execCmd.Run()
	if err != nil {
		c.t.Fatalf("failed to start container: %v", err)
	}
}

func (c *Container) Destroy(cli *Docker) {
	execCmd := exec.Command(cli.client, "rm", "-f", c.id)
	err := execCmd.Run()
	if err != nil {
		c.t.Fatalf("failed to destroy container: %v", err)
	}
}

func (c *Container) Exec(cli *Docker, cmd string) (string, string, error) {
	args := []string{"exec", c.id, "sh", "-c", cmd}
	execCmd := exec.Command(cli.client, args...)
	c.t.Logf("executing command: docker %s", strings.Join(execCmd.Args, " "))
	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr
	err := execCmd.Run()
	return stdout.String(), stderr.String(), err
}
