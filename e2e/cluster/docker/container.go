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

	name string
	t    *testing.T
}

func NewContainer(t *testing.T, name string) *Container {
	return &Container{
		name: name,
		t:    t,
	}
}

func (c *Container) GetName() string {
	return c.name
}

func (c *Container) WithImage(image string) *Container {
	c.Image = image
	return c
}

func (c *Container) WithECBinary(path string) *Container {
	path, err := filepath.Abs(path)
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

func (c *Container) WithTroubleshootDir() *Container {
	troubleshootPath, err := filepath.Abs("../operator/charts/embedded-cluster-operator/troubleshoot")
	if err != nil {
		c.t.Fatalf("failed to get absolute path to troubleshoot dir: %v", err)
	}
	c = c.WithVolume(fmt.Sprintf("%s:%s", troubleshootPath, "/automation/troubleshoot"))
	return c
}

func (c *Container) WithVolume(volume string) *Container {
	c.Volumes = append(c.Volumes, volume)
	return c
}

func (c *Container) WithPort(port string) *Container {
	c.Ports = append(c.Ports, port)
	return c
}

func (c *Container) Run() {
	execCmd := exec.Command(
		dockerBinPath(c.t),
		"run",
		"-d",
		"--privileged",
		"--restart=unless-stopped",
		"--hostname",
		c.name,
		"--name",
		c.name,
	)
	for _, volume := range c.Volumes {
		execCmd.Args = append(execCmd.Args, "-v", volume)
	}
	for _, port := range c.Ports {
		execCmd.Args = append(execCmd.Args, "-p", port)
	}
	execCmd.Args = append(execCmd.Args, c.Image)
	c.t.Logf("running container: %s", strings.Join(execCmd.Args, " "))
	output, err := execCmd.CombinedOutput()
	if err != nil {
		c.t.Fatalf("failed to run container: %v: %s", err, string(output))
	}
}

func (c *Container) Destroy() {
	execCmd := exec.Command(dockerBinPath(c.t), "rm", "-f", "--volumes", c.name)
	output, err := execCmd.CombinedOutput()
	if err != nil {
		c.t.Fatalf("failed to destroy container: %v: %s", err, string(output))
	}
}

func (c *Container) Exec(line []string, envs ...map[string]string) (string, string, error) {
	args := []string{"exec"}
	for k, v := range mergeMaps(envs...) {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	args = append(args, c.name, "sh", "-c", strings.Join(line, " "))
	execCmd := exec.Command(dockerBinPath(c.t), args...)
	c.t.Logf("executing command: %s", strings.Join(execCmd.Args, " "))
	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr
	err := execCmd.Run()
	return stdout.String(), stderr.String(), err
}

func (c *Container) CopyFile(src, dst string) (string, string, error) {
	args := []string{"cp", src, dst}
	execCmd := exec.Command(dockerBinPath(c.t), args...)
	c.t.Logf("executing command: %s", strings.Join(execCmd.Args, " "))
	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr
	err := execCmd.Run()
	return stdout.String(), stderr.String(), err
}

func (c *Container) WaitForSystemd() {
	timeout := time.After(2 * time.Minute)
	tick := time.Tick(time.Second)
	for {
		select {
		case <-timeout:
			stdout, stderr, err := c.Exec([]string{"systemctl status"})
			c.t.Fatalf("timeout waiting for systemd to start: %v: %s: %s", err, stdout, stderr)
		case <-tick:
			status, _, _ := c.Exec([]string{"systemctl is-system-running"})
			c.t.Logf("systemd stdout: %s", status)
			if strings.TrimSpace(status) == "running" {
				return
			}
		}
	}
}

func (c *Container) WaitForClockSync() {
	timeout := time.After(2 * time.Minute)
	tick := time.Tick(time.Second)
	for {
		select {
		case <-timeout:
			stdout, stderr, err := c.Exec([]string{"timedatectl show -p NTP -p NTPSynchronized"})
			c.t.Fatalf("timeout waiting for clock sync: %v: %s: %s", err, stdout, stderr)
		case <-tick:
			status, _, _ := c.Exec([]string{"timedatectl show -p NTP -p NTPSynchronized"})
			c.t.Logf("timedatectl stdout: %s", status)
			if strings.Contains(status, "NTP=yes") && strings.Contains(status, "NTPSynchronized=yes") {
				return
			}
		}
	}
}
