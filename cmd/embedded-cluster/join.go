package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"

	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/goods"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/preflights"
	pb "github.com/replicatedhq/embedded-cluster/pkg/progressbar"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
)

// JoinCommandResponse is the response from the kots api we use to fetch the k0s join token.
type JoinCommandResponse struct {
	K0sJoinCommand            string    `json:"k0sJoinCommand"`
	K0sToken                  string    `json:"k0sToken"`
	ClusterID                 uuid.UUID `json:"clusterID"`
	K0sUnsupportedOverrides   string    `json:"k0sUnsupportedOverrides"`
	EndUserK0sConfigOverrides string    `json:"endUserK0sConfigOverrides"`
}

// extractK0sConfigOverridePatch parses the provided override and returns a dig.Mapping that
// can be then applied on top a k0s configuration file to set both `api` and `storage` spec
// fields. All other fields in the override are ignored.
func (j JoinCommandResponse) extractK0sConfigOverridePatch(data []byte) (dig.Mapping, error) {
	config := dig.Mapping{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unable to unmarshal embedded config: %w", err)
	}
	result := dig.Mapping{}
	if api := config.DigMapping("config", "spec", "api"); len(api) > 0 {
		result.DigMapping("config", "spec")["api"] = api
	}
	if storage := config.DigMapping("config", "spec", "storage"); len(storage) > 0 {
		result.DigMapping("config", "spec")["storage"] = storage
	}
	return result, nil
}

// EndUserOverrides returns a dig.Mapping that can be applied on top of a k0s configuration.
// This patch is assembled based on the EndUserK0sConfigOverrides field.
func (j JoinCommandResponse) EndUserOverrides() (dig.Mapping, error) {
	return j.extractK0sConfigOverridePatch([]byte(j.EndUserK0sConfigOverrides))
}

// EmbeddedOverrides returns a dig.Mapping that can be applied on top of a k0s configuration.
// This patch is assembled based on the K0sUnsupportedOverrides field.
func (j JoinCommandResponse) EmbeddedOverrides() (dig.Mapping, error) {
	return j.extractK0sConfigOverridePatch([]byte(j.K0sUnsupportedOverrides))
}

// getJoinToken issues a request to the kots api to get the actual join command
// based on the short token provided by the user.
func getJoinToken(ctx context.Context, baseURL, shortToken string) (*JoinCommandResponse, error) {
	url := fmt.Sprintf("http://%s/api/v1/embedded-cluster/join?token=%s", baseURL, shortToken)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to get join token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	var command JoinCommandResponse
	if err := json.NewDecoder(resp.Body).Decode(&command); err != nil {
		return nil, fmt.Errorf("unable to decode response: %w", err)
	}
	return &command, nil
}

var joinCommand = &cli.Command{
	Name:  "join",
	Usage: "Join the current node to an existing cluster",
	Action: func(c *cli.Context) error {
		binname := defaults.BinaryName()
		if c.Args().Len() != 2 {
			return fmt.Errorf("usage: %s node join <url> <token>", binname)
		}
		if err := canRunJoin(c); err != nil {
			return err
		}
		loading := pb.Start()
		defer loading.Close()
		loading.Infof("Fetching join token remotely")
		jcmd, err := getJoinToken(c.Context, c.Args().Get(0), c.Args().Get(1))
		if err != nil {
			return err
		}
		metrics.ReportJoinStarted(c.Context, jcmd.ClusterID)
		loading.Infof("Materializing %s binaries", binname)
		if err := goods.Materialize(); err != nil {
			err := fmt.Errorf("unable to materialize binaries: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.ClusterID, err)
			return err
		}
		loading.Infof("Executing host preflights")
		if err := runHostPreflightsLocally(c); err != nil {
			err := fmt.Errorf("unable to run host preflights locally: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.ClusterID, err)
			return err
		}
		loading.Infof("Saving token to disk")
		if err := saveTokenToDisk(jcmd.K0sToken); err != nil {
			err := fmt.Errorf("unable to save token to disk: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.ClusterID, err)
			return err
		}
		loading.Infof("Installing %s binaries", binname)
		if err := installK0sBinary(); err != nil {
			err := fmt.Errorf("unable to install k0s binary: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.ClusterID, err)
			return err
		}
		loading.Infof("Joining node to cluster")
		if err := runK0sInstallCommand(jcmd.K0sJoinCommand); err != nil {
			err := fmt.Errorf("unable to join node to cluster: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.ClusterID, err)
			return err
		}
		loading.Infof("Applying configuration overrides")
		if err := applyJoinConfigurationOverrides(c, jcmd); err != nil {
			err := fmt.Errorf("unable to apply configuration overrides: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.ClusterID, err)
			return err
		}
		loading.Infof("Creating systemd unit file")
		if err := createSystemdUnitFile(jcmd.K0sJoinCommand); err != nil {
			err := fmt.Errorf("unable to create systemd unit file: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.ClusterID, err)
			return err
		}
		loading.Infof("Starting %s service", binname)
		if err := startK0sService(); err != nil {
			err := fmt.Errorf("unable to start service: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.ClusterID, err)
			return err
		}
		fpath := defaults.PathToConfig(".cluster-id")
		cid := jcmd.ClusterID.String()
		if err := os.WriteFile(fpath, []byte(cid), 0644); err != nil {
			err := fmt.Errorf("unable to write cluster id to disk: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.ClusterID, err)
			return err
		}
		metrics.ReportJoinSucceeded(c.Context, jcmd.ClusterID)
		loading.Infof("Join finished")
		return nil
	},
}

// applyJoinConfigurationOverrides applies both config overrides received from the kots api.
// Applies first the EmbeddedOverrides and then the EndUserOverrides.
func applyJoinConfigurationOverrides(c *cli.Context, jcmd *JoinCommandResponse) error {
	patch, err := jcmd.EmbeddedOverrides()
	if err != nil {
		return fmt.Errorf("unable to get embedded overrides: %w", err)
	}
	if len(patch) > 0 {
		if data, err := yaml.Marshal(patch); err != nil {
			return fmt.Errorf("unable to marshal embedded overrides: %w", err)
		} else if err := patchK0sConfig("/etc/k0s/k0s.yaml", string(data)); err != nil {
			return fmt.Errorf("unable to patch config with embedded data: %w", err)
		}
	}
	if patch, err = jcmd.EndUserOverrides(); err != nil {
		return fmt.Errorf("unable to get embedded overrides: %w", err)
	} else if len(patch) == 0 {
		return nil
	}
	if data, err := yaml.Marshal(patch); err != nil {
		return fmt.Errorf("unable to marshal embedded overrides: %w", err)
	} else if err := patchK0sConfig("/etc/k0s/k0s.yaml", string(data)); err != nil {
		return fmt.Errorf("unable to patch config with embedded data: %w", err)
	}
	return nil
}

// patchK0sConfig patches the created k0s config with the unsupported overrides passed in.
func patchK0sConfig(path string, patch string) error {
	if len(patch) == 0 {
		return nil
	}
	finalcfg := dig.Mapping{
		"apiVersion": "k0s.k0sproject.io/v1beta1",
		"kind":       "ClusterConfig",
		"metadata":   dig.Mapping{"name": defaults.BinaryName()},
	}
	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("unable to read node config: %w", err)
		}
		finalcfg = dig.Mapping{}
		if err := yaml.Unmarshal(data, &finalcfg); err != nil {
			return fmt.Errorf("unable to unmarshal node config: %w", err)
		}
	}
	k0sconfig := cluster.K0s{Config: finalcfg.Dup()}
	result, err := config.PatchK0sConfig(&k0sconfig, patch)
	if err != nil {
		return fmt.Errorf("unable to patch node config: %w", err)
	}
	if len(result.Config.DigMapping("spec", "api")) > 0 {
		finalcfg.DigMapping("spec")["api"] = result.Config.DigMapping("spec", "api")
	}
	if len(result.Config.DigMapping("spec", "storage")) > 0 {
		finalcfg.DigMapping("spec")["storage"] = result.Config.DigMapping("spec", "storage")
	}
	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to open node config file for writing: %w", err)
	}
	defer out.Close()
	if err := yaml.NewEncoder(out).Encode(finalcfg); err != nil {
		return fmt.Errorf("unable to write node config: %w", err)
	}
	return nil
}

// saveTokenToDisk saves the provided token in "/etc/k0s/join-token".
func saveTokenToDisk(token string) error {
	if err := os.MkdirAll("/etc/k0s", 0755); err != nil {
		return err
	}
	data := []byte(token)
	if err := os.WriteFile("/etc/k0s/join-token", data, 0644); err != nil {
		return err
	}
	return nil
}

// installK0sBinary saves the embedded k0s binary to disk under /usr/local/bin.
func installK0sBinary() error {
	in, err := os.Open(defaults.K0sBinaryPath())
	if err != nil {
		return fmt.Errorf("unable to open embedded k0s binary: %w", err)
	}
	defer in.Close()
	out, err := os.OpenFile("/usr/local/bin/k0s", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("unable to open k0s binary: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("unable to copy k0s binary: %w", err)
	}
	return nil
}

// startK0sService starts the k0s service.
func startK0sService() error {
	cmd := exec.Command("/usr/local/bin/k0s", "start")
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("service start failed:")
		fmt.Fprintf(os.Stderr, "%s\n", stderr.String())
		fmt.Fprintf(os.Stdout, "%s\n", stdout.String())
		return err
	}
	return nil
}

// canRunJoin checks if we can run the join command. Checks if we are running on linux,
// if we are root, and if a token has been provided through the command line.
func canRunJoin(c *cli.Context) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("join command is only supported on linux")
	}
	if os.Getuid() != 0 {
		return fmt.Errorf("join command must be run as root")
	}
	return nil
}

// createSystemdUnitFile links the k0s systemd unit file.
func createSystemdUnitFile(fullcmd string) error {
	dst := fmt.Sprintf("/etc/systemd/system/%s.service", defaults.BinaryName())
	if _, err := os.Stat(dst); err == nil {
		if err := os.Remove(dst); err != nil {
			return err
		}
	}
	src := "/etc/systemd/system/k0scontroller.service"
	if strings.Contains(fullcmd, "worker") {
		src = "/etc/systemd/system/k0sworker.service"
	}
	if err := os.Symlink(src, dst); err != nil {
		return err
	}
	cmd := exec.Command("systemctl", "daemon-reload")
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("systemctl reload failed:")
		fmt.Fprintf(os.Stderr, "%s\n", stderr.String())
		fmt.Fprintf(os.Stdout, "%s\n", stdout.String())
		return err
	}
	return nil
}

// runK0sInstallCommand runs the k0s install command as provided by the kots
// adm api.
func runK0sInstallCommand(fullcmd string) error {
	args := strings.Split(fullcmd, " ")
	args = append(args, "--token-file", "/etc/k0s/join-token")
	if strings.Contains(fullcmd, "controller") {
		args = append(args, "--disable-components", "konnectivity-server", "--enable-dynamic-config")
	}
	cmd := exec.Command(args[0], args[1:]...)
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("install failed:")
		fmt.Fprintf(os.Stderr, "%s\n", stderr.String())
		fmt.Fprintf(os.Stdout, "%s\n", stdout.String())
		return err
	}
	return nil
}

// runHostPreflightsLocally runs the embedded host preflights in the local node prior to
// node upgrade.
func runHostPreflightsLocally(c *cli.Context) error {
	logrus.Infof("Running host preflights locally")
	hpf, err := addons.NewApplier().HostPreflights()
	if err != nil {
		return fmt.Errorf("unable to read host preflights: %w", err)
	}
	if len(hpf.Collectors) == 0 && len(hpf.Analyzers) == 0 {
		logrus.Info("No host preflights found")
		return nil
	}
	out, err := preflights.RunLocal(c.Context, hpf)
	if err != nil {
		return fmt.Errorf("preflight failed: %w", err)
	}
	out.PrintTable()
	if out.HasFail() {
		return fmt.Errorf("preflights haven't passed on one or more hosts")
	}
	if !out.HasWarn() || c.Bool("no-prompt") {
		return nil
	}
	fmt.Println("Host preflights have warnings on one or more hosts")
	if !prompts.New().Confirm("Do you want to continue ?", false) {
		return fmt.Errorf("user aborted")
	}
	return nil
}
