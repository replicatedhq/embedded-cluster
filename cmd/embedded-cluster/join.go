package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/k0sproject/dig"
	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8syaml "sigs.k8s.io/yaml"

	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

// JoinCommandResponse is the response from the kots api we use to fetch the k0s join token.
type JoinCommandResponse struct {
	K0sJoinCommand            string    `json:"k0sJoinCommand"`
	K0sToken                  string    `json:"k0sToken"`
	ClusterID                 uuid.UUID `json:"clusterID"`
	K0sUnsupportedOverrides   string    `json:"k0sUnsupportedOverrides"`
	EndUserK0sConfigOverrides string    `json:"endUserK0sConfigOverrides"`
	MetricsBaseURL            string    `json:"metricsBaseURL"`
	AirgapRegistryAddress     string    `json:"airgapRegistryAddress"`
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
	url := fmt.Sprintf("https://%s/api/v1/embedded-cluster/join?token=%s", baseURL, shortToken)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create request: %w", err)
	}

	// this will generally be a self-signed certificate created by kurl-proxy
	insecureClient := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	resp, err := insecureClient.Do(req)
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
	Name:      "join",
	Usage:     fmt.Sprintf("Join the current node to a %s cluster", binName),
	ArgsUsage: "<url> <token>",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:   "airgap-bundle",
			Usage:  "Path to the airgap bundle. If set, the installation will be completed without internet access.",
			Hidden: true,
		},
		&cli.BoolFlag{
			Name:   "enable-ha",
			Usage:  "Enable high availability",
			Hidden: true,
		},
	},
	Before: func(c *cli.Context) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("node join command must be run as root")
		}
		if c.String("airgap-bundle") != "" {
			metrics.DisableMetrics()
		}
		os.Setenv("KUBECONFIG", defaults.PathToKubeConfig())
		return nil
	},
	Action: func(c *cli.Context) error {
		logrus.Debugf("checking if %s is already installed", binName)
		if installed, err := isAlreadyInstalled(); err != nil {
			return err
		} else if installed {
			logrus.Errorf("An installation has been detected on this machine.")
			logrus.Infof("If you want to reinstall you need to remove the existing installation")
			logrus.Infof("first. You can do this by running the following command:")
			logrus.Infof("\n  sudo ./%s reset\n", binName)
			return ErrNothingElseToAdd
		}

		if c.Args().Len() != 2 {
			return fmt.Errorf("usage: %s node join <url> <token>", binName)
		}

		logrus.Infof("Fetching join token remotely")
		jcmd, err := getJoinToken(c.Context, c.Args().Get(0), c.Args().Get(1))
		if err != nil {
			return fmt.Errorf("unable to get join token: %w", err)
		}

		if c.String("airgap-bundle") != "" {
			logrus.Debugf("checking airgap bundle matches binary")
			if err := checkAirgapMatches(c); err != nil {
				return err // we want the user to see the error message without a prefix
			}
		}

		metrics.ReportJoinStarted(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID)
		logrus.Infof("Materializing %s binaries", binName)
		if err := materializeFiles(c); err != nil {
			metrics.ReportJoinFailed(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID, err)
			return err
		}

		if err := RunHostPreflights(c); err != nil {
			err := fmt.Errorf("unable to run host preflights locally: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID, err)
			return err
		}

		logrus.Debugf("configuring network manager")
		if err := configureNetworkManager(c); err != nil {
			return fmt.Errorf("unable to configure network manager: %w", err)
		}

		logrus.Infof("Saving token to disk")
		if err := saveTokenToDisk(jcmd.K0sToken); err != nil {
			err := fmt.Errorf("unable to save token to disk: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID, err)
			return err
		}

		logrus.Infof("Installing %s binaries", binName)
		if err := installK0sBinary(); err != nil {
			err := fmt.Errorf("unable to install k0s binary: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID, err)
			return err
		}

		if jcmd.AirgapRegistryAddress != "" {
			if err := airgap.AddInsecureRegistry(jcmd.AirgapRegistryAddress); err != nil {
				err := fmt.Errorf("unable to add insecure registry: %w", err)
				metrics.ReportJoinFailed(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID, err)
				return err
			}
		}

		logrus.Infof("Joining node to cluster")
		if err := runK0sInstallCommand(jcmd.K0sJoinCommand); err != nil {
			err := fmt.Errorf("unable to join node to cluster: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID, err)
			return err
		}

		logrus.Infof("Applying configuration overrides")
		if err := applyJoinConfigurationOverrides(jcmd); err != nil {
			err := fmt.Errorf("unable to apply configuration overrides: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID, err)
			return err
		}

		logrus.Infof("Creating systemd unit files")
		if err := createSystemdUnitFiles(jcmd.K0sJoinCommand); err != nil {
			err := fmt.Errorf("unable to create systemd unit files: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID, err)
			return err
		}

		logrus.Infof("Starting %s service", binName)
		if err := startK0sService(); err != nil {
			err := fmt.Errorf("unable to start service: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID, err)
			return err
		}

		if c.Bool("enable-ha") {
			if err := waitForK0s(); err != nil {
				err := fmt.Errorf("unable to wait for node: %w", err)
				metrics.ReportJoinFailed(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID, err)
				return err
			}
			if err := maybeEnableHA(c.Context); err != nil {
				err := fmt.Errorf("unable to enable HA: %w", err)
				metrics.ReportJoinFailed(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID, err)
				return err
			}
		}

		metrics.ReportJoinSucceeded(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID)
		logrus.Infof("Join finished")
		return nil
	},
}

// applyJoinConfigurationOverrides applies both config overrides received from the kots api.
// Applies first the EmbeddedOverrides and then the EndUserOverrides.
func applyJoinConfigurationOverrides(jcmd *JoinCommandResponse) error {
	patch, err := jcmd.EmbeddedOverrides()
	if err != nil {
		return fmt.Errorf("unable to get embedded overrides: %w", err)
	}
	if len(patch) > 0 {
		if data, err := yaml.Marshal(patch); err != nil {
			return fmt.Errorf("unable to marshal embedded overrides: %w", err)
		} else if err := patchK0sConfig(
			defaults.PathToK0sConfig(), string(data),
		); err != nil {
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
	} else if err := patchK0sConfig(
		defaults.PathToK0sConfig(), string(data),
	); err != nil {
		return fmt.Errorf("unable to patch config with embedded data: %w", err)
	}
	return nil
}

// patchK0sConfig patches the created k0s config with the unsupported overrides passed in.
func patchK0sConfig(path string, patch string) error {
	if len(patch) == 0 {
		return nil
	}
	finalcfg := k0sconfig.ClusterConfig{
		ObjectMeta: metav1.ObjectMeta{Name: defaults.BinaryName()},
	}
	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("unable to read node config: %w", err)
		}
		finalcfg = k0sconfig.ClusterConfig{}
		if err := k8syaml.Unmarshal(data, &finalcfg); err != nil {
			return fmt.Errorf("unable to unmarshal node config: %w", err)
		}
	}
	result, err := config.PatchK0sConfig(finalcfg.DeepCopy(), patch)
	if err != nil {
		return fmt.Errorf("unable to patch node config: %w", err)
	}
	if result.Spec.API != nil {
		if finalcfg.Spec == nil {
			finalcfg.Spec = &k0sconfig.ClusterSpec{}
		}
		finalcfg.Spec.API = result.Spec.API
	}
	if result.Spec.Storage != nil {
		if finalcfg.Spec == nil {
			finalcfg.Spec = &k0sconfig.ClusterSpec{}
		}
		finalcfg.Spec.Storage = result.Spec.Storage
	}
	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to open node config file for writing: %w", err)
	}
	defer out.Close()
	data, err := k8syaml.Marshal(finalcfg)
	if err != nil {
		return fmt.Errorf("unable to marshal node config: %w", err)
	}
	if _, err := out.Write(data); err != nil {
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

// installK0sBinary moves the embedded k0s binary to its destination.
func installK0sBinary() error {
	ourbin := defaults.PathToEmbeddedClusterBinary("k0s")
	hstbin := defaults.K0sBinaryPath()
	if err := helpers.MoveFile(ourbin, hstbin); err != nil {
		return fmt.Errorf("unable to move k0s binary: %w", err)
	}
	return nil
}

// startK0sService starts the k0s service.
func startK0sService() error {
	if _, err := helpers.RunCommand(defaults.K0sBinaryPath(), "start"); err != nil {
		return fmt.Errorf("unable to start: %w", err)
	}
	return nil
}

func systemdUnitFileName() string {
	return fmt.Sprintf("/etc/systemd/system/%s.service", defaults.BinaryName())
}

// createSystemdUnitFiles links the k0s systemd unit file. this also creates a new
// systemd unit file for the local artifact mirror service.
func createSystemdUnitFiles(fullcmd string) error {
	dst := systemdUnitFileName()
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
	if _, err := helpers.RunCommand("systemctl", "daemon-reload"); err != nil {
		return err
	}
	return installAndEnableLocalArtifactMirror()
}

// runK0sInstallCommand runs the k0s install command as provided by the kots
// adm api.
func runK0sInstallCommand(fullcmd string) error {
	args := strings.Split(fullcmd, " ")
	args = append(args, "--token-file", "/etc/k0s/join-token")
	if strings.Contains(fullcmd, "controller") {
		args = append(args, "--disable-components", "konnectivity-server", "--enable-dynamic-config")
	}
	if _, err := helpers.RunCommand(args[0], args[1:]...); err != nil {
		return err
	}
	return nil
}

// maybeEnableHA checks if the cluster has more than 3 nodes and prompts the user
// to enable high availability if it is not already enabled.
func maybeEnableHA(ctx context.Context) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}
	embeddedclusterv1beta1.AddToScheme(kcli.Scheme())
	isHA, err := isHAEnabled(ctx, kcli)
	if err != nil {
		return fmt.Errorf("unable to check if HA is enabled: %w", err)
	}
	if isHA {
		return nil
	}
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("unable to get hostname: %w", err)
	}
	numNodes, err := waitForNode(ctx, kcli, hostname)
	if err != nil {
		return fmt.Errorf("unable to wait for node: %w", err)
	}
	if numNodes < 3 {
		return nil
	}
	shouldEnableHA := prompts.New().Confirm("Do you want to enable high availability?", false)
	if !shouldEnableHA {
		return nil
	}
	return enableHA(ctx, kcli)
}

// isHAEnabled checks if high availability is already enabled in the cluster.
func isHAEnabled(ctx context.Context, kcli client.Client) (bool, error) {
	installation, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return false, fmt.Errorf("unable to get latest installation: %w", err)
	}
	return installation.Spec.HighAvailability, nil
}

// waitForNode waits for the node to be registered in the k8s cluster.
// It returns the total number of nodes in the cluster.
func waitForNode(ctx context.Context, kcli client.Client, name string) (int, error) {
	loading := spinner.Start()
	defer loading.Close()
	loading.Infof("Waiting for %s node to join the cluster", defaults.BinaryName())
	backoff := wait.Backoff{Steps: 60, Duration: 5 * time.Second, Factor: 1.0, Jitter: 0.1}
	var nodes corev1.NodeList
	var lasterr error
	if err := wait.ExponentialBackoffWithContext(
		ctx, backoff, func(ctx context.Context) (bool, error) {
			if err := kcli.List(ctx, &nodes); err != nil {
				lasterr = fmt.Errorf("unable to list nodes: %v", err)
				return false, nil
			}
			for _, node := range nodes.Items {
				if node.Name == name {
					return true, nil
				}
			}
			lasterr = fmt.Errorf("node %s not found", name)
			return false, nil
		},
	); err != nil {
		if lasterr != nil {
			return 0, fmt.Errorf("timed out waiting for node %s: %w", name, lasterr)
		} else {
			return 0, fmt.Errorf("timed out waiting for node %s", name)
		}
	}
	loading.Infof("Node has joined the cluster!")
	return len(nodes.Items), nil
}

// enableHA enables high availability in the installation object
// and waits for the migration to be complete.
func enableHA(ctx context.Context, kcli client.Client) error {
	loading := spinner.Start()
	defer loading.Close()
	loading.Infof("Enabling high availability")
	installation, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return fmt.Errorf("unable to get latest installation: %w", err)
	}
	installation.Spec.HighAvailability = true
	if err := kcli.Update(ctx, installation); err != nil {
		return fmt.Errorf("unable to update installation: %w", err)
	}

	// TODO: how should we wait for HA migration to complete?

	loading.Infof("High availability enabled!")
	return nil
}
