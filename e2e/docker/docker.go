package docker

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type Container struct {
	Image   string
	Volumes []string
	Ports   []string

	id string
	t  *testing.T
}

func DockerBinPath(t *testing.T) string {
	path, err := exec.LookPath("docker")
	if err != nil {
		t.Fatalf("failed to find docker in path: %v", err)
	}
	return path
}

func NewContainer(t *testing.T) *Container {
	return &Container{
		id: generateID(),
		t:  t,
	}
}

func (c *Container) GetID() string {
	return c.id
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
	return "/usr/local/bin/embedded-cluster"
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
	return "/assets/license.yaml"
}

func (c *Container) WithScripts() *Container {
	dir, err := filepath.Abs("scripts")
	if err != nil {
		c.t.Fatalf("failed to get absolute path to scripts dir: %v", err)
	}
	scripts, err := os.ReadDir(dir)
	if err != nil {
		c.t.Fatalf("fail to read scripts dir: %v", err)
	}
	for _, script := range scripts {
		c = c.WithVolume(fmt.Sprintf("%s:%s", filepath.Join(dir, script.Name()), c.GetScriptPath(script.Name())))
	}
	return c
}

func (c *Container) GetScriptPath(script string) string {
	return fmt.Sprintf("/usr/local/bin/%s", script)
}

func (c *Container) WithVolume(volume string) *Container {
	c.Volumes = append(c.Volumes, volume)
	return c
}

func (c *Container) WithPort(port string) *Container {
	c.Ports = append(c.Ports, port)
	return c
}

func (c *Container) Start() {
	execCmd := exec.Command(
		DockerBinPath(c.t),
		"run",
		"--rm",
		"-d",
		"--privileged",
		"--cgroupns=host",
		"--name",
		c.id,
	)
	for _, volume := range c.Volumes {
		execCmd.Args = append(execCmd.Args, "-v", volume)
	}
	for _, port := range c.Ports {
		execCmd.Args = append(execCmd.Args, "-p", port)
	}
	execCmd.Args = append(execCmd.Args, c.Image)
	c.t.Logf("starting container: %s", strings.Join(execCmd.Args, " "))
	output, err := execCmd.CombinedOutput()
	if err != nil {
		c.t.Fatalf("failed to start container: %v: %s", err, string(output))
	}
}

func (c *Container) Destroy() {
	execCmd := exec.Command(DockerBinPath(c.t), "rm", "-f", c.id)
	output, err := execCmd.CombinedOutput()
	if err != nil {
		c.t.Fatalf("failed to destroy container: %v: %s", err, string(output))
	}
}

func (c *Container) Exec(cmd string) (string, string, error) {
	args := []string{"exec", c.id, "sh", "-c", cmd}
	execCmd := exec.Command(DockerBinPath(c.t), args...)
	c.t.Logf("executing command: %s", strings.Join(execCmd.Args, " "))
	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr
	err := execCmd.Run()
	return stdout.String(), stderr.String(), err
}

func (c *Container) CopyFile(src, dst string) (string, string, error) {
	args := []string{"cp", src, dst}
	execCmd := exec.Command(DockerBinPath(c.t), args...)
	c.t.Logf("executing command: %s", strings.Join(execCmd.Args, " "))
	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr
	err := execCmd.Run()
	return stdout.String(), stderr.String(), err
}

func (c *Container) WaitForSystemd() {
	timeout := time.After(30 * time.Second)
	tick := time.Tick(time.Second)
	for {
		select {
		case <-timeout:
			c.t.Fatalf("timeout waiting for systemd to start")
		case <-tick:
			status, stderr, err := c.Exec("systemctl is-system-running")
			c.t.Logf("systemd status: %s, err: %v, stderr: %s", status, err, stderr)
			if strings.TrimSpace(status) == "running" {
				return
			}
		}
	}
}

func NewNode(t *testing.T, distro string) *Container {
	c := NewContainer(t).
		WithImage(fmt.Sprintf("replicated/ec-distro:%s", distro)).
		WithVolume("/var/lib/k0s").
		WithPort("30003:30003").
		WithScripts().
		WithECBinary()
	if licensePath := os.Getenv("LICENSE_PATH"); licensePath != "" {
		t.Logf("using license %s", licensePath)
		c = c.WithLicense(licensePath)
	}
	c.Start()
	c.WaitForSystemd()
	return c
}
