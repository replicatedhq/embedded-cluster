package main

import (
	"context"
	"fmt"

	"dagger/embedded-cluster/internal/dagger"
)

// CIItem is the name of the 1Password item for the CI secrets.
const CIItem = "EC CI"

// Provisions a new CMX VM for E2E testing.
//
// This creates a fresh VM with the specified configuration and waits for it to be ready.
// The VM is automatically configured with SSH access and networking.
//
// Example:
//
//	dagger call with-one-password --service-account=env:OP_SERVICE_ACCOUNT_TOKEN \
//	  test-provision-vm --name="my-test-vm"
func (m *EmbeddedCluster) TestProvisionVM(
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
	// SSH user
	// +default="ec-e2e-test"
	sshUser string,
) (*CMXInstance, error) {
	// Get CMX API token and SSH key from 1Password
	cmxToken := m.mustResolveSecret(nil, CIItem, "CMX_REPLICATED_API_TOKEN")
	sshKey := m.mustResolveSecret(nil, CIItem, "CMX_SSH_PRIVATE_KEY")

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

	instance := &CMXInstance{
		VmID:        string(vmID),
		Name:        vmName,
		NetworkID:   networkID,
		SSHEndpoint: sshEndpoint,
		SSHPort:     directSSHPort,
		SSHUser:     sshUser,
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
