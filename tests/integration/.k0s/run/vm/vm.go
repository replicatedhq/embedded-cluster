package vm

import (
	"fmt"
	"log"
	"path"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

const (
	DefaultDistro  = "debian-bookworm"
	DefaultDataDir = "/var/lib/embedded-cluster"
)

type CreateOptions struct {
	Distro  string
	DataDir string
}

type VM struct {
	Resource   *dockertest.Resource
	volumeName string
}

func (vm *VM) Close() (finalErr error) {
	if vm.Resource != nil {
		finalErr = vm.Resource.Close()
	}
	if vm.volumeName != "" {
		err := pool.Client.RemoveVolumeWithOptions(docker.RemoveVolumeOptions{
			Name:  vm.volumeName,
			Force: true,
		})
		if err != nil && err != docker.ErrNoSuchVolume {
			finalErr = fmt.Errorf("failed to remove volume: %w", err)
		}
	}
	return finalErr
}

// Create creates a virtual machine using dockertest with the replicated/ec-distro:debian-bookworm image.
// It returns the dockertest resource representing the container.
// If any error occurs during vm creation, it fails the test immediately.
func Create(
	name string, k0sBinary string, testBinary string, opts *CreateOptions,
) (*VM, error) {
	// Set default values if not provided
	if opts == nil {
		opts = &CreateOptions{}
	}
	if opts.Distro == "" {
		opts.Distro = DefaultDistro
	}
	if opts.DataDir == "" {
		opts.DataDir = DefaultDataDir
	}

	volumeName := fmt.Sprintf("%s-%d", name, time.Now().UnixNano())

	// Create and run the container with the specified image
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Name:       name,
		Hostname:   name,
		Repository: "replicated/ec-distro",
		Tag:        opts.Distro,
		Privileged: true,
		Mounts: []string{
			fmt.Sprintf("%s:/%s", volumeName, opts.DataDir),       // fix permissions errors
			fmt.Sprintf("%s:/tmp/k0s", k0sBinary),                 // access k0s binary
			fmt.Sprintf("%s:/usr/local/bin/k0s.test", testBinary), // run test
		},
	}, func(config *docker.HostConfig) {
		// Set restart policy to unless-stopped
		config.RestartPolicy = docker.RestartPolicy{
			Name: "unless-stopped",
		}
	})
	if err != nil {
		return nil, fmt.Errorf("could not start container: %w", err)
	}

	vm := &VM{
		Resource:   resource,
		volumeName: volumeName,
	}

	// Set a timeout for the container (not in the original Makefile but useful for tests)
	resource.Expire(900) // 15 minutes

	// Give the container a moment to initialize
	err = pool.Retry(func() error {
		// Run a basic command to check if the container is ready
		exitCode, err := resource.Exec([]string{"systemctl", "status"}, dockertest.ExecOptions{})
		if err != nil {
			return err
		} else if exitCode != 0 {
			return fmt.Errorf("sysctl status failed with exit code %d", exitCode)
		}
		return nil
	})
	if err != nil {
		// If there was an error during container startup, purge it and fail the test
		if purgeErr := vm.Close(); purgeErr != nil {
			log.Printf("Failed to purge container: %s", purgeErr)
		}
		return nil, fmt.Errorf("container failed to initialize: %w", err)
	}

	// Move the k0s binary to the data directory
	exitCode, err := resource.Exec([]string{
		"sh", "-c", fmt.Sprintf("mkdir -p %s && cp /tmp/k0s %s",
			path.Join(opts.DataDir, "bin"),
			path.Join(opts.DataDir, "bin/k0s"))},
		dockertest.ExecOptions{})
	if err != nil {
		// If there was an error during container startup, purge it and fail the test
		if purgeErr := vm.Close(); purgeErr != nil {
			log.Printf("Failed to purge container: %s", purgeErr)
		}
		return nil, fmt.Errorf("move k0s binary failed: %w", err)
	} else if exitCode != 0 {
		// If there was an error during container startup, purge it and fail the test
		if purgeErr := vm.Close(); purgeErr != nil {
			log.Printf("Failed to purge container: %s", purgeErr)
		}
		return nil, fmt.Errorf("move k0s binary failed with exit code %d", exitCode)
	}

	return vm, nil
}
