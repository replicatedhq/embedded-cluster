package configutils

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"go.uber.org/multierr"
)

// sysctlConfigPath is the path to the sysctl config file that is used to configure the embedded
// cluster. This could have been a constant but we want to be able to override it for testing
// purposes.
var sysctlConfigPath = "/etc/sysctl.d/99-embedded-cluster.conf"

var modulesLoadConfigPath = "/etc/modules-load.d/99-embedded-cluster.conf"

//go:embed static/sysctl.d/99-embedded-cluster.conf
var embeddedClusterSysctlConf []byte

//go:embed static/modules-load.d/99-embedded-cluster.conf
var embeddedClusterModulesConf []byte

// ConfigureSysctl writes the sysctl config file for the embedded cluster and reloads the sysctl
// configuration. This function has a distinct behavior: if the sysctl binary does not exist it
// returns an error but if it fails to lay down the sysctl config on disk it simply returns nil.
// NOTE: do not run this after the cluster has already been installed as it may revert sysctl
// settings set by k0s and its extensions.
func ConfigureSysctl() error {
	if _, err := exec.LookPath("sysctl"); err != nil {
		return fmt.Errorf("find sysctl binary: %w", err)
	}

	if err := sysctlConfig(); err != nil {
		return fmt.Errorf("materialize sysctl config: %w", err)
	}

	if _, err := helpers.RunCommand("sysctl", "--system"); err != nil {
		return fmt.Errorf("configure sysctl: %w", err)
	}
	return nil
}

// sysctlConfig writes the embedded sysctl config to the /etc/sysctl.d directory.
func sysctlConfig() error {
	if err := os.MkdirAll(filepath.Dir(sysctlConfigPath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	if err := os.WriteFile(sysctlConfigPath, embeddedClusterSysctlConf, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

// ConfigureKernelModules writes the kernel modules config file and ensures the kernel modules are
// loaded that are listed in the file.
func ConfigureKernelModules() error {
	if _, err := exec.LookPath("modprobe"); err != nil {
		return fmt.Errorf("find modprobe binary: %w", err)
	}

	if err := kernelModulesConfig(); err != nil {
		return fmt.Errorf("materialize kernel modules config: %w", err)
	}

	if err := ensureKernelModulesLoaded(); err != nil {
		return fmt.Errorf("ensure kernel modules are loaded: %w", err)
	}
	return nil
}

// kernelModulesConfig writes the embedded kernel modules config to the /etc/modules-load.d
// directory.
func kernelModulesConfig() error {
	if err := os.MkdirAll(filepath.Dir(modulesLoadConfigPath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	if err := os.WriteFile(modulesLoadConfigPath, embeddedClusterModulesConf, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

// ensureKernelModulesLoaded ensures the kernel modules are loaded by iterating over the modules in
// the config file and calling modprobe for each one.
func ensureKernelModulesLoaded() (finalErr error) {
	scanner := bufio.NewScanner(bytes.NewReader(embeddedClusterModulesConf))
	for scanner.Scan() {
		module := strings.TrimSpace(scanner.Text())
		if module != "" && !strings.HasPrefix(module, "#") {
			if err := modprobe(module); err != nil {
				err = fmt.Errorf("modprobe %s: %w", module, err)
				finalErr = multierr.Append(finalErr, err)
			}
		}
	}
	return
}

func modprobe(module string) error {
	_, err := helpers.RunCommand("modprobe", module)
	return err
}
