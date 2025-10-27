package hostutils

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/replicatedhq/embedded-cluster/cmd/installer/goods"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers/systemd"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
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

// systemdConfigPath is the path to the systemd unit files directory.
var systemdConfigPath = "/etc/systemd/system"

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
	if err := helpers.WriteFile(sysctlConfigPath, embeddedClusterSysctlConf, 0644); err != nil {
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

	if err := helpers.WriteFile(configPath, []byte(config.String()), 0644); err != nil {
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
	if err := helpers.WriteFile(modulesLoadConfigPath, embeddedClusterModulesConf, 0644); err != nil {
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

// CreateSystemdUnitFiles links the k0s systemd unit file. this also creates a new
// systemd unit file for the local artifact mirror service.
func (h *HostUtils) CreateSystemdUnitFiles(ctx context.Context, logger logrus.FieldLogger, rc runtimeconfig.RuntimeConfig, hostname string, isWorker bool) error {
	dst := systemdUnitFileName()
	if _, err := os.Lstat(dst); err == nil {
		if err := os.Remove(dst); err != nil {
			return err
		}
	}
	src := fmt.Sprintf("%s/k0scontroller.service", systemdConfigPath)
	if isWorker {
		src = fmt.Sprintf("%s/k0sworker.service", systemdConfigPath)
	}
	if proxy := rc.ProxySpec(); proxy != nil {
		if err := ensureProxyConfig(fmt.Sprintf("%s.d", src), proxy.HTTPProxy, proxy.HTTPSProxy, proxy.NoProxy); err != nil {
			return fmt.Errorf("unable to create proxy config: %w", err)
		}
	}
	if hostname != "" {
		if err := ensureAutopilotConfig(fmt.Sprintf("%s.d", src), hostname); err != nil {
			return fmt.Errorf("unable to create autopilot hostname config: %w", err)
		}
	}
	logger.Debugf("linking %s to %s", src, dst)
	if err := os.Symlink(src, dst); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	if _, err := helpers.RunCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("unable to get reload systemctl daemon: %w", err)
	}
	if err := h.installAndEnableLocalArtifactMirror(ctx, logger, rc); err != nil {
		return fmt.Errorf("unable to install and enable local artifact mirror: %w", err)
	}
	return nil
}

func systemdUnitFileName() string {
	return fmt.Sprintf("%s/%s.service", systemdConfigPath, runtimeconfig.AppSlug())
}

// ensureProxyConfig creates a new http-proxy.conf configuration file. The file is saved in the
// systemd directory (/etc/systemd/system/k0s{controller,worker}.service.d/).
func ensureProxyConfig(servicePath string, httpProxy string, httpsProxy string, noProxy string) error {
	// create the directory
	if err := os.MkdirAll(servicePath, 0755); err != nil {
		return fmt.Errorf("unable to create directory: %w", err)
	}

	// create and write the file
	content := fmt.Sprintf(`[Service]
Environment="HTTP_PROXY=%s"
Environment="HTTPS_PROXY=%s"
Environment="NO_PROXY=%s"`, httpProxy, httpsProxy, noProxy)

	err := helpers.WriteFile(filepath.Join(servicePath, "http-proxy.conf"), []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("unable to create and write proxy file: %w", err)
	}

	return nil
}

// installAndEnableLocalArtifactMirror installs and enables the local artifact mirror. This
// service is responsible for serving on localhost, through http, all files that are used
// during a cluster upgrade.
func (h *HostUtils) installAndEnableLocalArtifactMirror(ctx context.Context, logger logrus.FieldLogger, rc runtimeconfig.RuntimeConfig) error {
	materializer := goods.NewMaterializer(rc)
	if err := materializer.LocalArtifactMirrorUnitFile(); err != nil {
		return fmt.Errorf("failed to materialize artifact mirror unit: %w", err)
	}
	if err := h.WriteLocalArtifactMirrorDropInFile(rc); err != nil {
		return fmt.Errorf("failed to write local artifact mirror environment file: %w", err)
	}
	if _, err := helpers.RunCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("unable to get reload systemctl daemon: %w", err)
	}
	if _, err := helpers.RunCommand("systemctl", "enable", "local-artifact-mirror"); err != nil {
		return fmt.Errorf("unable to enable the local artifact mirror service: %w", err)
	}
	logger.Debugf("Starting local artifact mirror")
	if _, err := helpers.RunCommand("systemctl", "start", "local-artifact-mirror"); err != nil {
		return fmt.Errorf("unable to start the local artifact mirror: %w", err)
	}
	if err := waitForLocalArtifactMirror(ctx); err != nil {
		return fmt.Errorf("unable to wait for the local artifact mirror: %w", err)
	}
	logger.Debugf("Local artifact mirror started")
	return nil
}

func waitForLocalArtifactMirror(ctx context.Context) error {
	consecutiveSuccesses := 0
	requiredSuccesses := 3
	maxAttempts := 30
	checkInterval := 2 * time.Second

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		_, err := helpers.RunCommand("systemctl", "status", "local-artifact-mirror")
		if err == nil {
			consecutiveSuccesses++
			if consecutiveSuccesses >= requiredSuccesses {
				return nil
			}
		} else {
			consecutiveSuccesses = 0
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(checkInterval):
			continue
		}
	}

	return lastErr
}

const (
	localArtifactMirrorDropInFileContents = `[Service]
Environment="LOCAL_ARTIFACT_MIRROR_PORT=%d"
Environment="LOCAL_ARTIFACT_MIRROR_DATA_DIR=%s"
# Empty ExecStart= will clear out the previous ExecStart value
ExecStart=
ExecStart=%s serve
`
)

func (h *HostUtils) WriteLocalArtifactMirrorDropInFile(rc runtimeconfig.RuntimeConfig) error {
	contents := fmt.Sprintf(
		localArtifactMirrorDropInFileContents,
		rc.LocalArtifactMirrorPort(),
		rc.EmbeddedClusterHomeDirectory(),
		rc.PathToEmbeddedClusterBinary("local-artifact-mirror"),
	)
	err := systemd.WriteDropInFile("local-artifact-mirror.service", "embedded-cluster.conf", []byte(contents))
	if err != nil {
		return fmt.Errorf("write drop-in file: %w", err)
	}
	return nil
}

// ensureAutopilotConfig creates a new autopilot-hostname.conf configuration file. The file is saved in the
// systemd directory (/etc/systemd/system/k0s{controller,worker}.service.d/).
func ensureAutopilotConfig(servicePath string, hostname string) error {
	if err := os.MkdirAll(servicePath, 0755); err != nil {
		return fmt.Errorf("unable to create directory: %w", err)
	}

	content := fmt.Sprintf(`[Service]
Environment="AUTOPILOT_HOSTNAME=%s"`, hostname)

	err := helpers.WriteFile(filepath.Join(servicePath, "autopilot-hostname.conf"), []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("unable to create and write autopilot hostname file: %w", err)
	}

	return nil
}
