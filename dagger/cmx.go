package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dagger/embedded-cluster/internal/dagger"
)

// CMXInstance wraps the CMX VM instance.
type CMXInstance struct {
	// VM ID
	VmID string
	// VM name
	Name string
	// Network ID
	NetworkID string
	// Private IP address
	PrivateIP string
	// SSH endpoint
	SSHEndpoint string
	// SSH port
	SSHPort int
	// SSH user
	SSHUser string
	// +private
	SSHKey *dagger.Secret
	// +private
	CMXToken *dagger.Secret
}

// sshClient returns a container with openssh-client installed and the SSH key configured.
// The key is mounted at /root/.ssh/id_rsa with proper permissions and formatting.
func (i *CMXInstance) sshClient() *dagger.Container {
	return dag.Container().
		From("ubuntu:24.04").
		WithEnvVariable("DEBIAN_FRONTEND", "noninteractive").
		WithExec([]string{"apt-get", "update"}).
		WithExec([]string{"apt-get", "install", "-y", "openssh-client"}).
		WithMountedSecret("/tmp/key", i.SSHKey).
		WithExec([]string{"mkdir", "-p", "/root/.ssh"}).
		// Ensure the key ends with exactly one newline (required by OpenSSH)
		WithExec([]string{"sh", "-c", "sed -e '$a\\' /tmp/key > /root/.ssh/id_rsa"}).
		WithExec([]string{"chmod", "600", "/root/.ssh/id_rsa"})
}

// waitForSSH waits for SSH to become available on the VM.
func (i *CMXInstance) waitForSSH(ctx context.Context) error {
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Prepare SSH client container once to avoid repeating apt operations on every retry
	sshClient := i.sshClient()

	for attempt := 1; ; attempt++ {
		select {
		case <-timeout:
			return fmt.Errorf("timed out waiting for ssh")
		case <-ticker.C:
			_, err := sshClient.
				WithEnvVariable("ATTEMPT", fmt.Sprintf("%d", attempt)). // cache bust
				WithExec([]string{
					"ssh",
					"-i", "/root/.ssh/id_rsa",
					"-o", "StrictHostKeyChecking=no",
					"-o", "BatchMode=yes",
					"-o", "ConnectTimeout=5",
					"-p", fmt.Sprintf("%d", i.SSHPort),
					fmt.Sprintf("%s@%s", i.SSHUser, i.SSHEndpoint),
					"uptime",
				}).
				Stdout(ctx)

			if err == nil {
				return nil
			}
			// Continue waiting on error
		}
	}
}

// discoverPrivateIP discovers the private IP address of the VM.
func (i *CMXInstance) discoverPrivateIP(ctx context.Context) (string, error) {
	stdout, err := i.sshClient().
		WithExec([]string{
			"ssh",
			"-i", "/root/.ssh/id_rsa",
			"-o", "StrictHostKeyChecking=no",
			"-o", "BatchMode=yes",
			"-p", fmt.Sprintf("%d", i.SSHPort),
			fmt.Sprintf("%s@%s", i.SSHUser, i.SSHEndpoint),
			"hostname -I",
		}).
		Stdout(ctx)
	if err != nil {
		return "", fmt.Errorf("run hostname command: %w", err)
	}

	// Look for an IP starting with "10."
	for ip := range strings.FieldsSeq(stdout) {
		if strings.HasPrefix(ip, "10.") {
			return ip, nil
		}
	}

	return "", fmt.Errorf("no private ip found starting with 10")
}

// RunCommand runs a command on the CMX VM.
//
// Commands are executed with sudo and the PATH is set to include /usr/local/bin.
//
// Example:
//
//	dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
//	  test-provision-vm run-command --command="ls,-la,/tmp"
func (i *CMXInstance) RunCommand(
	ctx context.Context,
	// Command to run (as array of strings)
	command []string,
) (string, error) {
	// Build the full SSH command
	fullCmd := append([]string{"sudo", "PATH=$PATH:/usr/local/bin"}, command...)
	cmdStr := strings.Join(fullCmd, " ")

	stdout, err := i.sshClient().
		WithExec([]string{
			"ssh",
			"-i", "/root/.ssh/id_rsa",
			"-o", "StrictHostKeyChecking=no",
			"-o", "BatchMode=yes",
			"-p", fmt.Sprintf("%d", i.SSHPort),
			fmt.Sprintf("%s@%s", i.SSHUser, i.SSHEndpoint),
			cmdStr,
		}).
		Stdout(ctx)

	if err != nil {
		return "", fmt.Errorf("run command failed: %w", err)
	}

	return stdout, nil
}

// ExposePort exposes a port on the VM and returns the public hostname.
//
// Example:
//
//	dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
//	  e2e-init e2e-provision-vm expose-port --port=30000 --protocol="https"
func (i *CMXInstance) ExposePort(
	ctx context.Context,
	// Port to expose
	port int,
	// Protocol (defaults to "https")
	// +optional
	protocol string,
) (string, error) {
	// Set default protocol
	if protocol == "" {
		protocol = "https"
	}

	// Expose the port using Replicated module
	portExpose := dag.
		Replicated(i.CMXToken).
		VMExposePort(i.VmID, port, protocol)

	hostname, err := portExpose.Hostname(ctx)
	if err != nil {
		return "", fmt.Errorf("get hostname: %w", err)
	}

	return hostname, nil
}

// Cleanup removes the CMX VM.
//
// This should be called to clean up VMs after testing is complete.
//
// Example:
//
//	dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
//	  e2e-init e2e-provision-vm cleanup
func (i *CMXInstance) Cleanup(ctx context.Context) (string, error) {
	result, err := dag.
		Replicated(i.CMXToken).
		VMRemove(ctx, dagger.ReplicatedVMRemoveOpts{
			VMID: i.VmID,
		})

	if err != nil {
		return "", fmt.Errorf("remove vm: %w", err)
	}

	return result, nil
}
