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
	ecv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
	K0sJoinCommand            string                 `json:"k0sJoinCommand"`
	K0sToken                  string                 `json:"k0sToken"`
	ClusterID                 uuid.UUID              `json:"clusterID"`
	K0sUnsupportedOverrides   string                 `json:"k0sUnsupportedOverrides"`
	EndUserK0sConfigOverrides string                 `json:"endUserK0sConfigOverrides"`
	MetricsBaseURL            string                 `json:"metricsBaseURL"`
	AirgapRegistryAddress     string                 `json:"airgapRegistryAddress"`
	Proxy                     *ecv1beta1.ProxySpec   `json:"proxy"`
	Network                   *ecv1beta1.NetworkSpec `json:"network"`
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
		&cli.BoolFlag{
			Name:  "skip-host-preflights",
			Usage: "Skip host preflight checks. This is not recommended unless you are sure your system is compatible.",
			Value: false,
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

		logrus.Debugf("fetching join token remotely")
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
		logrus.Debugf("materializing %s binaries", binName)
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

		logrus.Debugf("saving token to disk")
		if err := saveTokenToDisk(jcmd.K0sToken); err != nil {
			err := fmt.Errorf("unable to save token to disk: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID, err)
			return err
		}

		logrus.Debugf("installing %s binaries", binName)
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

		logrus.Debugf("creating systemd unit files")
		// both controller and worker nodes will have 'worker' in the join command
		if err := createSystemdUnitFiles(!strings.Contains(jcmd.K0sJoinCommand, "controller"), jcmd.Proxy); err != nil {
			err := fmt.Errorf("unable to create systemd unit files: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID, err)
			return err
		}

		logrus.Debugf("joining node to cluster")
		if err := runK0sInstallCommand(jcmd.K0sJoinCommand); err != nil {
			err := fmt.Errorf("unable to join node to cluster: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID, err)
			return err
		}

		logrus.Debugf("overriding network configuration")
		if err := applyNetworkConfiguration(jcmd); err != nil {
			err := fmt.Errorf("unable to apply network configuration: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID, err)
		}

		logrus.Debugf("applying configuration overrides")
		if err := applyJoinConfigurationOverrides(jcmd); err != nil {
			err := fmt.Errorf("unable to apply configuration overrides: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID, err)
			return err
		}

		logrus.Debugf("starting %s service", binName)
		if err := startK0sService(); err != nil {
			err := fmt.Errorf("unable to start service: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID, err)
			return err
		}

		if err := waitForK0s(); err != nil {
			err := fmt.Errorf("unable to wait for node: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID, err)
			return err
		}

		if !strings.Contains(jcmd.K0sJoinCommand, "controller") {
			metrics.ReportJoinSucceeded(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID)
			logrus.Debugf("worker node join finished")
			return nil
		}

		kcli, err := kubeutils.KubeClient()
		if err != nil {
			err := fmt.Errorf("unable to get kube client: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID, err)
			return err
		}
		hostname, err := os.Hostname()
		if err != nil {
			err := fmt.Errorf("unable to get hostname: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID, err)
			return err
		}
		if err := waitForNode(c.Context, kcli, hostname); err != nil {
			err := fmt.Errorf("unable to wait for node: %w", err)
			metrics.ReportJoinFailed(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID, err)
			return err
		}

		if c.Bool("enable-ha") {
			if err := maybeEnableHA(c.Context, kcli); err != nil {
				err := fmt.Errorf("unable to enable high availability: %w", err)
				metrics.ReportJoinFailed(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID, err)
				return err
			}
		}

		metrics.ReportJoinSucceeded(c.Context, jcmd.MetricsBaseURL, jcmd.ClusterID)
		logrus.Debugf("controller node join finished")
		return nil
	},
}

func applyNetworkConfiguration(jcmd *JoinCommandResponse) error {
	if jcmd.Network != nil {
		clusterSpec := k0sconfig.DefaultClusterConfig()
		clusterSpec.Spec.Network.PodCIDR = jcmd.Network.PodCIDR
		clusterSpec.Spec.Network.ServiceCIDR = jcmd.Network.ServiceCIDR
		clusterSpecYaml, err := k8syaml.Marshal(clusterSpec)

		if err != nil {
			return fmt.Errorf("unable to marshal cluster spec: %w", err)
		}
		err = os.WriteFile(defaults.PathToK0sConfig(), clusterSpecYaml, 0644)
		if err != nil {
			return fmt.Errorf("unable to write cluster spec to /etc/k0s/k0s.yaml: %w", err)
		}

		// remove /var/lib/k0s/pki/server.crt and /var/lib/k0s/pki/server.key so that they are generated with the correct service IP
		err = os.Remove("/var/lib/k0s/pki/server.crt")
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("unable to remove /var/lib/k0s/pki/server.crt: %w", err)
		}

		err = os.Remove("/var/lib/k0s/pki/server.key")
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("unable to remove /var/lib/k0s/pki/server.key: %w", err)
		}
	}
	return nil
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

func waitForNode(ctx context.Context, kcli client.Client, hostname string) error {
	loading := spinner.Start()
	defer loading.Close()
	loading.Infof("Waiting for node to join the cluster")
	if err := kubeutils.WaitForControllerNode(ctx, kcli, hostname); err != nil {
		return fmt.Errorf("unable to wait for node: %w", err)
	}
	loading.Infof("Node has joined the cluster!")
	return nil
}

func maybeEnableHA(ctx context.Context, kcli client.Client) error {
	canEnableHA, err := canEnableHA(ctx, kcli)
	if err != nil {
		return fmt.Errorf("unable to check if HA can be enabled: %w", err)
	}
	if !canEnableHA {
		return nil
	}
	logrus.Info("")
	logrus.Info("When adding a third controller node, you have the option to enable high availability. This will migrate the data so that it is replicated across cluster nodes. Once enabled, you must maintain at least three controller nodes.")
	logrus.Info("")
	shouldEnableHA := prompts.New().Confirm("Do you want to enable high availability?", false)
	if !shouldEnableHA {
		return nil
	}
	logrus.Info("")
	return enableHA(ctx, kcli)
}

// canEnableHA checks if high availability can be enabled in the cluster.
func canEnableHA(ctx context.Context, kcli client.Client) (bool, error) {
	installation, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return false, fmt.Errorf("unable to get latest installation: %w", err)
	}
	if installation.Spec.HighAvailability {
		return false, nil
	}
	if err := kcli.Get(ctx, types.NamespacedName{Name: ecRestoreStateCMName, Namespace: "embedded-cluster"}, &corev1.ConfigMap{}); err == nil {
		return false, nil // cannot enable HA during a restore
	} else if !errors.IsNotFound(err) {
		return false, fmt.Errorf("unable to get restore state configmap: %w", err)
	}
	ncps, err := kubeutils.NumOfControlPlaneNodes(ctx, kcli)
	if err != nil {
		return false, fmt.Errorf("unable to check control plane nodes: %w", err)
	}
	return ncps >= 3, nil
}

// enableHA enables high availability in the installation object
// and waits for the migration to be complete.
func enableHA(ctx context.Context, kcli client.Client) error {
	loading := spinner.Start()
	defer loading.Close()
	loading.Infof("Enabling high availability")
	in, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return fmt.Errorf("unable to get latest installation: %w", err)
	}
	in.Spec.HighAvailability = true
	if err := kcli.Update(ctx, in); err != nil {
		return fmt.Errorf("unable to update installation: %w", err)
	}
	if err := kubeutils.WaitForHAInstallation(ctx, kcli); err != nil {
		return fmt.Errorf("unable to wait for ha installation: %w", err)
	}
	loading.Infof("High availability enabled!")
	return nil
}
