package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dagger/embedded-cluster/internal/dagger"
)

const (
	SSHUser = "ec-e2e-test"
	DataDir = "/var/lib/embedded-cluster"
)

// Provisions a new CMX VM for E2E testing.
//
// This creates a fresh VM with the specified configuration and waits for it to be ready.
// The VM is automatically configured with SSH access and networking.
//
// Example:
//
//	dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
//	  provision-cmx-vm --name="my-test-vm"
func (m *EmbeddedCluster) ProvisionCmxVm(
	ctx context.Context,
	// Name for the VM
	// +default="ec-e2e-test"
	name string,
	// OS distribution
	// +default="ubuntu"
	distribution string,
	// Distribution version
	// +default="22.04"
	version string,
	// Instance type
	// +default="r1.medium"
	instanceType string,
	// Disk size in GB
	// +default=50
	diskSize int,
	// How long to wait for VM to be ready
	// +default="10m"
	wait string,
	// TTL for the VM
	// +default="2h"
	ttl string,
	// CMX API token
	// +optional
	cmxToken *dagger.Secret,
	// SSH key
	// +optional
	sshKey *dagger.Secret,
) (*CmxInstance, error) {
	// Get CMX API token and SSH key from 1Password if not provided
	cmxToken = m.mustResolveSecret(cmxToken, "CMX_REPLICATED_API_TOKEN")
	sshKey = m.mustResolveSecret(sshKey, "CMX_SSH_PRIVATE_KEY")

	// Create VM using Replicated Dagger module
	vms, err := dag.
		Replicated(cmxToken).
		VMCreate(
			ctx,
			dagger.ReplicatedVMCreateOpts{
				Name:         name,
				Wait:         wait,
				TTL:          ttl,
				Distribution: distribution,
				Version:      version,
				Count:        1,
				Disk:         diskSize,
				InstanceType: instanceType,
			},
		)
	if err != nil {
		return nil, fmt.Errorf("create vm: %w", err)
	}

	// Get the first VM
	if len(vms) == 0 {
		return nil, fmt.Errorf("no VMs created")
	}
	vm := vms[0]

	return m.cmxVmToCmxInstance(ctx, &vm, cmxToken, sshKey)
}

// WithCmxVm connects to an existing CMX VM by ID.
//
// This queries the CMX API to get the VM details and creates a CmxInstance.
// Unlike ProvisionCmxVm, this does not create a new VM - it connects to one that already exists.
//
// Example:
//
//	dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
//	  with-cmx-vm --vm-id="abc123"
func (m *EmbeddedCluster) WithCmxVm(
	ctx context.Context,
	// VM ID
	vmId string,
	// SSH user
	// +default="ec-e2e-test"
	sshUser string,
	// CMX API token
	// +optional
	cmxToken *dagger.Secret,
	// SSH key
	// +optional
	sshKey *dagger.Secret,
) (*CmxInstance, error) {
	// Get CMX API token and SSH key from 1Password if not provided
	cmxToken = m.mustResolveSecret(cmxToken, "CMX_REPLICATED_API_TOKEN")
	sshKey = m.mustResolveSecret(sshKey, "CMX_SSH_PRIVATE_KEY")

	// List all VMs and find the one with matching ID
	vms, err := dag.
		Replicated(cmxToken).
		VMList(ctx)
	if err != nil {
		return nil, fmt.Errorf("list vms: %w", err)
	}

	// Find VM with matching ID
	var vm *dagger.ReplicatedVM
	for _, v := range vms {
		id, err := v.ItemID(ctx)
		if err != nil {
			continue
		}
		if string(id) == vmId {
			vm = &v
			break
		}
	}

	if vm == nil {
		return nil, fmt.Errorf("vm with id %s not found", vmId)
	}

	return m.cmxVmToCmxInstance(ctx, vm, cmxToken, sshKey)
}

func (m *EmbeddedCluster) cmxVmToCmxInstance(ctx context.Context, vm *dagger.ReplicatedVM, cmxToken *dagger.Secret, sshKey *dagger.Secret) (*CmxInstance, error) {
	// Get VM details
	vmID, err := vm.ItemID(ctx)
	if err != nil {
		return nil, fmt.Errorf("get vm id: %w", err)
	}

	vmName, err := vm.Name(ctx)
	if err != nil {
		return nil, fmt.Errorf("get vm name: %w", err)
	}

	networkID, err := vm.NetworkID(ctx)
	if err != nil {
		return nil, fmt.Errorf("get network id: %w", err)
	}

	sshEndpoint, err := vm.DirectSshendpoint(ctx)
	if err != nil {
		return nil, fmt.Errorf("get ssh endpoint: %w", err)
	}

	directSSHPort, err := vm.DirectSshport(ctx)
	if err != nil {
		return nil, fmt.Errorf("get direct ssh port: %w", err)
	}

	instance := &CmxInstance{
		VmID:        string(vmID),
		Name:        vmName,
		NetworkID:   networkID,
		SSHEndpoint: sshEndpoint,
		SSHPort:     directSSHPort,
		SSHUser:     SSHUser,
		SSHKey:      sshKey,
		CMXToken:    cmxToken,
	}

	// Wait for SSH to be available
	if err := instance.waitForSSH(ctx); err != nil {
		return nil, fmt.Errorf("wait for ssh: %w", err)
	}

	// Discover private IP
	privateIP, err := instance.discoverPrivateIP(ctx)
	if err != nil {
		return nil, fmt.Errorf("discover private ip: %w", err)
	}
	instance.PrivateIP = privateIP

	return instance, nil
}

// CmxInstance wraps the CMX VM instance.
type CmxInstance struct {
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

// String returns a string representation of the CMX instance.
func (i *CmxInstance) String() string {
	return fmt.Sprintf("CmxInstance{VmID: %s, SSHEndpoint: %s, SSHPort: %d}", i.VmID, i.SSHEndpoint, i.SSHPort)
}

// sshClient returns a container with openssh-client installed and the SSH key configured.
// The key is mounted at /root/.ssh/id_rsa with proper permissions and formatting.
func (i *CmxInstance) sshClient() *dagger.Container {
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
func (i *CmxInstance) waitForSSH(ctx context.Context) error {
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timed out waiting for ssh")
		case <-ticker.C:
			_, err := i.Command(`uptime`).Stdout(ctx)
			if err == nil {
				return nil
			}
			// Continue waiting on error
		}
	}
}

// discoverPrivateIP discovers the private IP address of the VM.
func (i *CmxInstance) discoverPrivateIP(ctx context.Context) (string, error) {
	stdout, err := i.Command(`hostname -I`).Stdout(ctx)
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

// Command returns a dagger container that runs a command on the CMX VM.
//
// Commands are executed with sudo and the PATH is set to include /usr/local/bin.
// The returned container can be further customized before calling .Stdout() or other methods.
//
// Example:
//
//	dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
//	  with-cmx-vm --vm-id 8a2a66ef \
//	  command --command="ls -la /tmp" stdout
func (i *CmxInstance) Command(
	// Command to run
	command string,
) *dagger.Container {
	return i.CommandWithEnv(command, nil)
}

// CommandWithEnv runs a command with custom environment variables set on the remote system.
//
// Environment variables are passed as KEY=value pairs and will be available to the command.
//
// Example:
//
//	dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
//	  with-cmx-vm --vm-id 8a2a66ef \
//	  command-with-env --command="kubectl get pods" --env="KUBECONFIG=/path/to/kubeconfig" stdout
func (i *CmxInstance) CommandWithEnv(
	// Command to run
	command string,
	// Environment variables (e.g., "KUBECONFIG=/path/to/config")
	// +optional
	env []string,
) *dagger.Container {
	env = append([]string{
		// Use \$PATH to escape the variable so it survives bash -c and expands on the remote side
		fmt.Sprintf("PATH=$PATH:%s/bin", DataDir),
		fmt.Sprintf("KUBECONFIG=%s", fmt.Sprintf("%s/k0s/pki/admin.conf", DataDir)),
	}, env...)

	// Build environment variable string
	envVars := strings.Join(env, " ")

	// Escape single quotes in the command
	command = strings.ReplaceAll(command, `'`, `'"'"'`)

	// Build the full remote command
	// Use 'env' to set environment variables for sudo to avoid secure_path issues
	remoteCmd := fmt.Sprintf(`sudo -E env %s bash -c '%s'`, envVars, command)

	// Build SSH command
	sshCmd := []string{
		"ssh",
		"-i", "/root/.ssh/id_rsa",
		"-o", "StrictHostKeyChecking=no",
		"-o", "BatchMode=yes",
		"-p", fmt.Sprintf("%d", i.SSHPort),
		fmt.Sprintf("%s@%s", i.SSHUser, i.SSHEndpoint),
		remoteCmd,
	}

	// Return container with SSH exec
	// We use double quotes around the remote command so variables can expand
	return i.sshClient().
		WithEnvVariable("CACHE_BUSTER", time.Now().String()).
		WithExec(sshCmd)
}

// UploadFile uploads file content to a path on the VM using SCP.
//
// This uses SCP to transfer the file to /tmp first (avoiding permission issues),
// then moves it to the final destination using sudo.
//
// Example:
//
//	dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
//	  with-cmx-vm --vm-id 8a2a66ef \
//	  upload-file --path=/tmp/myfile.txt --file=/path/to/file.txt
func (i *CmxInstance) UploadFile(
	ctx context.Context,
	// Destination path on the VM
	path string,
	// File to upload
	file *dagger.File,
) error {
	// Create a temporary file in the container with the content
	tempPath := "/tmp/upload-file"
	tmpDest := fmt.Sprintf("/tmp/upload-%d", time.Now().UnixNano())

	container := i.sshClient().
		WithFile(tempPath, file).
		WithEnvVariable("CACHE_BUSTER", time.Now().String())

	// Use SCP to upload the file to /tmp on the VM (user has write access)
	scpCmd := []string{
		"scp",
		"-i", "/root/.ssh/id_rsa",
		"-o", "StrictHostKeyChecking=no",
		"-o", "BatchMode=yes",
		"-P", fmt.Sprintf("%d", i.SSHPort),
		tempPath,
		fmt.Sprintf("%s@%s:%s", i.SSHUser, i.SSHEndpoint, tmpDest),
	}

	if _, err := container.WithExec(scpCmd).Stdout(ctx); err != nil {
		return fmt.Errorf("scp upload to %s: %w", tmpDest, err)
	}

	// Move the file to the final destination using sudo
	moveCmd := fmt.Sprintf("mv %s %s", tmpDest, path)
	if _, err := i.Command(moveCmd).Stdout(ctx); err != nil {
		return fmt.Errorf("move file to %s: %w", path, err)
	}

	return nil
}

// ExposePort exposes a port on the VM and returns the public hostname.
//
// Example:
//
//	dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
//	  with-cmx-vm --vm-id 8a2a66ef \
//	  expose-port --port=30000 --protocol="https"
func (i *CmxInstance) ExposePort(
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
//	  with-cmx-vm --vm-id 8a2a66ef cleanup
func (i *CmxInstance) Cleanup(ctx context.Context) (string, error) {
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

// ApplyAirgapNetworkPolicy applies a network policy to block internet access for airgap testing.
//
// This calls the Replicated module's NetworkUpdatePolicy to set the VM's network to airgap mode,
// which prevents the VM from accessing external networks during airgap installation tests.
func (i *CmxInstance) ApplyAirgapNetworkPolicy(ctx context.Context) error {
	_, err := dag.
		Replicated(i.CMXToken).
		NetworkUpdatePolicy(ctx, "airgap", dagger.ReplicatedNetworkUpdatePolicyOpts{
			NetworkID: i.NetworkID,
		})

	if err != nil {
		return fmt.Errorf("update network policy to airgap: %w", err)
	}

	return nil
}

// InstallKotsCli installs the kubectl-kots CLI if not already present.
// This is needed for validation commands that use kubectl kots.
func (i *CmxInstance) InstallKotsCli(ctx context.Context) error {
	// Check if kubectl-kots is already installed
	_, err := i.Command("command -v kubectl-kots").Stdout(ctx)
	if err == nil {
		// Already installed
		return nil
	}

	// Install curl if needed
	_, err = i.Command("command -v curl").Stdout(ctx)
	if err != nil {
		installCurlCmd := "apt-get update && apt-get install -y curl"
		if _, err := i.Command(installCurlCmd).Stdout(ctx); err != nil {
			return fmt.Errorf("install curl: %w", err)
		}
	}

	// Get AdminConsole version from embedded-cluster
	versionOutput, err := i.Command("embedded-cluster-smoke-test-staging-app version").Stdout(ctx)
	if err != nil {
		return fmt.Errorf("get embedded-cluster version: %w", err)
	}

	// Parse version from output like "AdminConsole: v1.117.2-ec.2"
	// We want to extract "1.117.2" (without the 'v' prefix and '-ec.2' suffix)
	getVersionCmd := `embedded-cluster-smoke-test-staging-app version | grep AdminConsole | awk '{print substr($4,2)}' | cut -d'-' -f1`
	kotsVersion, err := i.Command(getVersionCmd).Stdout(ctx)
	if err != nil {
		return fmt.Errorf("parse kots version: %w", err)
	}

	kotsVersion = strings.TrimSpace(kotsVersion)
	if kotsVersion == "" {
		return fmt.Errorf("could not determine kots version from: %s", versionOutput)
	}

	// Download and install kots CLI
	installKotsCmd := fmt.Sprintf(`curl --retry 5 -fL -o /tmp/kotsinstall.sh "https://kots.io/install/%s" && chmod +x /tmp/kotsinstall.sh && /tmp/kotsinstall.sh`, kotsVersion)
	if _, err := i.Command(installKotsCmd).Stdout(ctx); err != nil {
		return fmt.Errorf("install kots cli: %w", err)
	}

	return nil
}

// PrepareRelease downloads embedded-cluster release from replicated.app
// and prepares it for installation. This matches how customers get the binary.
//
// The method downloads the release tarball, extracts it, and places the binary and
// license file in the expected locations for installation.
func (i *CmxInstance) PrepareRelease(
	ctx context.Context,
	// Installation scenario (online, airgap)
	scenario string,
	// App version to download
	appVersion string,
	// License file
	licenseFile *dagger.File,
) error {
	// Get license content as plain text for passing to install command
	_, licenseID, channelID, err := parseLicense(ctx, licenseFile)
	if err != nil {
		return fmt.Errorf("parse license: %w", err)
	}

	// Download embedded-cluster release from replicated.app
	releaseURL := fmt.Sprintf("https://ec-e2e-replicated-app.testcluster.net/embedded/embedded-cluster-smoke-test-staging-app/%s/%s", channelID, appVersion)

	if scenario == "airgap" {
		releaseURL = fmt.Sprintf("%s?airgap=true", releaseURL)
	}

	downloadCmd := fmt.Sprintf(`curl --retry 5 --retry-all-errors -fL -o /tmp/ec-release.tgz "%s" -H "Authorization: %s"`, releaseURL, licenseID)
	if _, err := i.Command(downloadCmd).Stdout(ctx); err != nil {
		return fmt.Errorf("download release: %w", err)
	}

	// Extract release tarball
	if _, err := i.Command(`tar xzf /tmp/ec-release.tgz -C /tmp`).Stdout(ctx); err != nil {
		return fmt.Errorf("extract release: %w", err)
	}

	// Create assets directory
	if _, err := i.Command(`mkdir -p /assets`).Stdout(ctx); err != nil {
		return fmt.Errorf("create assets directory: %w", err)
	}

	// Move binary to /usr/local/bin
	moveBinaryCmd := `mv /tmp/embedded-cluster-smoke-test-staging-app /usr/local/bin/embedded-cluster-smoke-test-staging-app`
	if _, err := i.Command(moveBinaryCmd).Stdout(ctx); err != nil {
		return fmt.Errorf("move binary: %w", err)
	}

	// Move license to /assets
	moveLicenseCmd := `mv /tmp/license.yaml /assets/license.yaml`
	if _, err := i.Command(moveLicenseCmd).Stdout(ctx); err != nil {
		return fmt.Errorf("move license: %w", err)
	}

	if scenario == "airgap" {
		// Move airgap bundle to /assets
		moveAirgapBundleCmd := `mv /tmp/embedded-cluster-smoke-test-staging-app.airgap /assets/embedded-cluster-smoke-test-staging-app.airgap`
		if _, err := i.Command(moveAirgapBundleCmd).Stdout(ctx); err != nil {
			return fmt.Errorf("move airgap bundle: %w", err)
		}
	}

	// Install kots CLI if not already installed
	if err := i.InstallKotsCli(ctx); err != nil {
		return fmt.Errorf("install kots cli: %w", err)
	}

	return nil
}

// InstallHeadless performs a headless (CLI) installation without Playwright.
//
// This method downloads the release, optionally uploads a config file, builds the
// installation command with appropriate flags, and runs the installation with a
// 30-minute timeout. It supports both online and airgap scenarios.
//
// Example:
//
//	dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
//	  with-cmx-vm --vm-id 8a2a66ef \
//	  install-headless --scenario online \
//	    --app-version=appver-dev-xpXCTO \
//	    --license-file ./local-dev/ethan-dev-3-license.yaml \
//	    --config-values-file ./assets/config-values.yaml
func (i *CmxInstance) InstallHeadless(
	ctx context.Context,
	// Installation scenario (online, airgap)
	scenario string,
	// App version to install
	appVersion string,
	// License file
	licenseFile *dagger.File,
	// Config values file
	configValuesFile *dagger.File,
) (*InstallResult, error) {
	// Upload config file if provided
	if err := i.UploadFile(ctx, "/assets/config-values.yaml", configValuesFile); err != nil {
		return nil, fmt.Errorf("upload config file: %w", err)
	}

	// Build install command
	installCmd := `ENABLE_V3=1 /usr/local/bin/embedded-cluster-smoke-test-staging-app install ` +
		`--license /assets/license.yaml ` +
		`--target linux ` +
		`--headless ` +
		`--config-values /assets/config-values.yaml ` +
		`--admin-console-password password ` +
		`--yes`

	// Add airgap bundle for airgap scenario
	if scenario == "airgap" {
		installCmd = fmt.Sprintf(`%s --airgap-bundle /assets/embedded-cluster-smoke-test-staging-app.airgap`, installCmd)
	}

	// Run installation command with timeout
	// Note: We use a simple approach here - start the command and wait for it to complete
	// The command itself may take up to 30 minutes
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	stdout, err := i.Command(installCmd).Stdout(ctx)
	if err != nil {
		return &InstallResult{
			Success:         false,
			InstallationLog: stdout,
		}, fmt.Errorf("installation failed: %w", err)
	}

	// Installation succeeded
	return &InstallResult{
		Success:         true,
		KubeconfigPath:  fmt.Sprintf("%s/k0s/pki/admin.conf", DataDir),
		InstallationLog: stdout,
	}, nil
}
