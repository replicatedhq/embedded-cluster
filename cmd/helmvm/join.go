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
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/helmvm/pkg/defaults"
	"github.com/replicatedhq/helmvm/pkg/goods"
	"github.com/replicatedhq/helmvm/pkg/metrics"
	pb "github.com/replicatedhq/helmvm/pkg/progressbar"
)

// JoinCommandResponse is the response from the kots api we use to fetch the k0s join
// token. It returns the actual command we need to run and also the cluster ID.
type JoinCommandResponse struct {
	K0sJoinCommand string    `json:"k0sJoinCommand"`
	K0sToken       string    `json:"k0sToken"`
	ClusterID      uuid.UUID `json:"clusterID"`
}

// getJoinToken issues a request to the kots api to get the actual join command
// based on the short token provided by the user.
func getJoinToken(ctx context.Context, baseURL, shortToken string) (*JoinCommandResponse, error) {
	url := fmt.Sprintf("%s/helmvm/join?token=%s", baseURL, shortToken)
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
	if len(args) < 2 {
		return fmt.Errorf("unable to run install command: invalid command")
	}
	cmd := exec.Command("/usr/local/bin/k0s", args[1:]...)
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
