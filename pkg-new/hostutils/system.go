package hostutils

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"go.uber.org/multierr"
)

// sysctlConfigPath is the path to the sysctl config file that is used to configure the embedded
// cluster. This could have been a constant but we want to be able to override it for testing
// purposes.
var sysctlConfigPath = "/etc/sysctl.d/99-embedded-cluster.conf"

// dynamicSysctlConfigPath is the path to the dynamic sysctl config file that is used to configure
// the embedded cluster.
const dynamicSysctlConfigPath = "/etc/sysctl.d/99-dynamic-embedded-cluster.conf"

// modulesLoadConfigPath is the path to the kernel modules config file that is used to configure
// the embedded cluster.
const modulesLoadConfigPath = "/etc/modules-load.d/99-embedded-cluster.conf"

//go:embed static/sysctl.d/99-embedded-cluster.conf
var embeddedClusterSysctlConf []byte

//go:embed static/modules-load.d/99-embedded-cluster.conf
var embeddedClusterModulesConf []byte

// dynamicSysctlConstraints are the constraints that are used to generate the dynamic sysctl
// config file.
var dynamicSysctlConstraints = []sysctlConstraint{
	// Increase inotify limits only if they are currently lower,
	// ensuring proper operation of applications that monitor filesystem events.
	{key: "fs.inotify.max_user_instances", value: 1024, operator: sysctlOperatorMin},
	{key: "fs.inotify.max_user_watches", value: 65536, operator: sysctlOperatorMin},
}

type sysctlOperator string

const (
	sysctlOperatorMin sysctlOperator = "min"
	sysctlOperatorMax sysctlOperator = "max"
)

type sysctlConstraint struct {
	key      string
	value    int64
	operator sysctlOperator
}

type sysctlValueGetter func(key string) (int64, error)

// ConfigureSysctl writes the sysctl config files for the embedded cluster and reloads the sysctl configuration.
// NOTE: do not run this after the cluster has already been installed as it may revert sysctl
// settings set by k0s and its extensions.
func (h *HostUtils) ConfigureSysctl() error {
	if _, err := exec.LookPath("sysctl"); err != nil {
		return fmt.Errorf("find sysctl binary: %w", err)
	}

	if err := sysctlConfig(); err != nil {
		return fmt.Errorf("materialize sysctl config: %w", err)
	}

	if err := dynamicSysctlConfig(); err != nil {
		return fmt.Errorf("materialize dynamic sysctl config: %w", err)
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

// dynamicSysctlConfig generates a dynamic sysctl config file based on current system values
// and our constraints.
func dynamicSysctlConfig() error {
	return generateDynamicSysctlConfig(getCurrentSysctlValue, dynamicSysctlConfigPath)
}

// generateDynamicSysctlConfig is the testable version of dynamicSysctlConfig that accepts
// a custom sysctl value getter and config path.
func generateDynamicSysctlConfig(getter sysctlValueGetter, configPath string) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	var config strings.Builder
	config.WriteString("# Dynamic sysctl configuration for embedded-cluster\n")
	config.WriteString("# This file is generated based on system values\n\n")

	for _, constraint := range dynamicSysctlConstraints {
		currentValue, err := getter(constraint.key)
		if err != nil {
			return fmt.Errorf("check current value for %s: %w", constraint.key, err)
		}

		needsUpdate := false
		switch constraint.operator {
		case sysctlOperatorMin:
			needsUpdate = currentValue < constraint.value
		case sysctlOperatorMax:
			needsUpdate = currentValue > constraint.value
		}

		if needsUpdate {
			config.WriteString(fmt.Sprintf("%s = %d\n", constraint.key, constraint.value))
		}
	}

	if err := os.WriteFile(configPath, []byte(config.String()), 0644); err != nil {
		return fmt.Errorf("write dynamic config file: %w", err)
	}
	return nil
}

// getCurrentSysctlValue reads the current value of a sysctl parameter
func getCurrentSysctlValue(key string) (int64, error) {
	out, err := helpers.RunCommand("sysctl", "-n", key)
	if err != nil {
		return 0, fmt.Errorf("get sysctl value: %w", err)
	}

	value, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse sysctl value: %w", err)
	}

	return value, nil
}

// ConfigureKernelModules writes the kernel modules config file and ensures the kernel modules are
// loaded that are listed in the file.
func (h *HostUtils) ConfigureKernelModules() error {
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
