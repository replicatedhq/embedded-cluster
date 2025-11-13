package k0s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	apv1b2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/artifacts"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/autopilot"
	"github.com/replicatedhq/embedded-cluster/pkg-new/domains"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8syaml "sigs.k8s.io/yaml"
)

const (
	k0sBinPath = "/usr/local/bin/k0s"
)

var _ K0sInterface = (*K0s)(nil)

type K0s struct {
}

func New() K0sInterface {
	if _clientFactory != nil {
		return _clientFactory()
	}
	return &K0s{}
}

var (
	_clientFactory ClientFactory
)

type ClientFactory func() K0sInterface

func SetClientFactory(fn ClientFactory) {
	_clientFactory = fn
}

// GetStatus calls the k0s status command and returns information about system init, PID, k0s role,
// kubeconfig and similar.
func (k *K0s) GetStatus(ctx context.Context) (*K0sStatus, error) {
	if _, err := os.Stat(k0sBinPath); err != nil {
		return nil, err
	}

	// get k0s status json
	out, err := exec.CommandContext(ctx, k0sBinPath, "status", "-o", "json").Output()
	if err != nil {
		return nil, err
	}
	var status K0sStatus
	err = json.Unmarshal(out, &status)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

// Install runs the k0s install command and waits for it to finish. If no configuration
// is found one is generated.
func (k *K0s) Install(rc runtimeconfig.RuntimeConfig, hostname string) error {
	ourbin := rc.PathToEmbeddedClusterBinary("k0s")
	hstbin := runtimeconfig.K0sBinaryPath
	if err := helpers.MoveFile(ourbin, hstbin); err != nil {
		return fmt.Errorf("unable to move k0s binary: %w", err)
	}

	nodeIP, err := netutils.FirstValidAddress(rc.NetworkInterface())
	if err != nil {
		return fmt.Errorf("unable to find first valid address: %w", err)
	}
	flags, err := config.InstallFlags(rc, nodeIP, hostname)
	if err != nil {
		return fmt.Errorf("unable to get install flags: %w", err)
	}
	if _, err := helpers.RunCommand(hstbin, flags...); err != nil {
		return fmt.Errorf("unable to install: %w", err)
	}
	if _, err := helpers.RunCommand(hstbin, "start"); err != nil {
		return fmt.Errorf("unable to start: %w", err)
	}
	return nil
}

// IsInstalled checks if the embedded cluster is already installed by looking for
// the k0s configuration file existence.
func (k *K0s) IsInstalled() (bool, error) {
	_, err := os.Stat(runtimeconfig.K0sConfigPath)
	if err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	}

	return false, fmt.Errorf("unable to check if already installed: %w", err)
}

// NewK0sConfig creates a new k0sv1beta1.ClusterConfig object from the input parameters.
func (k *K0s) NewK0sConfig(networkInterface string, isAirgap bool, podCIDR string, serviceCIDR string, eucfg *ecv1beta1.Config, mutate func(*k0sv1beta1.ClusterConfig) error) (*k0sv1beta1.ClusterConfig, error) {
	var embCfgSpec *ecv1beta1.ConfigSpec
	if embCfg := release.GetEmbeddedClusterConfig(); embCfg != nil {
		embCfgSpec = &embCfg.Spec
	}

	domains := domains.GetDomains(embCfgSpec, release.GetChannelRelease())
	cfg := config.RenderK0sConfig(domains.ProxyRegistryDomain)

	address, err := netutils.FirstValidAddress(networkInterface)
	if err != nil {
		return nil, fmt.Errorf("unable to find first valid address: %w", err)
	}
	cfg.Spec.API.Address = address
	cfg.Spec.Storage.Etcd.PeerAddress = address

	cfg.Spec.Network.PodCIDR = podCIDR
	cfg.Spec.Network.ServiceCIDR = serviceCIDR

	if mutate != nil {
		if err := mutate(cfg); err != nil {
			return nil, err
		}
	}

	cfg, err = applyUnsupportedOverrides(cfg, eucfg)
	if err != nil {
		return nil, fmt.Errorf("unable to apply unsupported overrides: %w", err)
	}

	if isAirgap {
		// update the k0s config to install with airgap
		airgap.SetAirgapConfig(cfg)
	}

	return cfg, nil
}

// WriteK0sConfig creates a new k0s.yaml configuration file. The file is saved in the
// global location (as returned by runtimeconfig.K0sConfigPath). If a file already sits
// there, this function returns an error.
func (k *K0s) WriteK0sConfig(ctx context.Context, cfg *k0sv1beta1.ClusterConfig) error {
	cfgpath := runtimeconfig.K0sConfigPath
	if _, err := os.Stat(cfgpath); err == nil {
		return fmt.Errorf("configuration file already exists")
	}
	if err := os.MkdirAll(filepath.Dir(cfgpath), 0755); err != nil {
		return fmt.Errorf("unable to create directory: %w", err)
	}

	// This is necessary to install the previous version of k0s in e2e tests
	// TODO: remove this once the previous version is > 1.29
	unstructured, err := helpers.K0sClusterConfigTo129Compat(cfg)
	if err != nil {
		return fmt.Errorf("unable to convert cluster config to 1.29 compat: %w", err)
	}
	data, err := k8syaml.Marshal(unstructured)
	if err != nil {
		return fmt.Errorf("unable to marshal config: %w", err)
	}
	if err := os.WriteFile(cfgpath, data, 0600); err != nil {
		return fmt.Errorf("unable to write config file: %w", err)
	}

	return nil
}

// applyUnsupportedOverrides applies overrides to the k0s configuration. Applies the
// overrides embedded into the binary and then the ones provided by the user (--overrides).
func applyUnsupportedOverrides(cfg *k0sv1beta1.ClusterConfig, eucfg *ecv1beta1.Config) (*k0sv1beta1.ClusterConfig, error) {
	embcfg := release.GetEmbeddedClusterConfig()
	if embcfg != nil {
		// Apply vendor k0s overrides
		vendorOverrides := embcfg.Spec.UnsupportedOverrides.K0s
		var err error
		cfg, err = config.PatchK0sConfig(cfg, vendorOverrides, false)
		if err != nil {
			return nil, fmt.Errorf("unable to patch k0s config: %w", err)
		}
	}

	if eucfg != nil {
		// Apply end user k0s overrides
		endUserOverrides := eucfg.Spec.UnsupportedOverrides.K0s
		var err error
		cfg, err = config.PatchK0sConfig(cfg, endUserOverrides, false)
		if err != nil {
			return nil, fmt.Errorf("unable to apply overrides: %w", err)
		}
	}

	return cfg, nil
}

// PatchK0sConfig patches the created k0s config with the unsupported overrides passed in.
func (k *K0s) PatchK0sConfig(path string, patch string) error {
	if len(patch) == 0 {
		return nil
	}
	finalcfg := k0sv1beta1.ClusterConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "k0s"},
	}
	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("unable to read node config: %w", err)
		}
		finalcfg = k0sv1beta1.ClusterConfig{}
		if err := k8syaml.Unmarshal(data, &finalcfg); err != nil {
			return fmt.Errorf("unable to unmarshal node config: %w", err)
		}
	}
	result, err := config.PatchK0sConfig(finalcfg.DeepCopy(), patch, false)
	if err != nil {
		return fmt.Errorf("unable to patch node config: %w", err)
	}
	if result.Spec.API != nil {
		if finalcfg.Spec == nil {
			finalcfg.Spec = &k0sv1beta1.ClusterSpec{}
		}
		finalcfg.Spec.API = result.Spec.API
	}
	if result.Spec.Storage != nil {
		if finalcfg.Spec == nil {
			finalcfg.Spec = &k0sv1beta1.ClusterSpec{}
		}
		finalcfg.Spec.Storage = result.Spec.Storage
	}
	if result.Spec.WorkerProfiles != nil {
		if finalcfg.Spec == nil {
			finalcfg.Spec = &k0sv1beta1.ClusterSpec{}
		}
		finalcfg.Spec.WorkerProfiles = result.Spec.WorkerProfiles
	}
	// This is necessary to install the previous version of k0s in e2e tests
	// TODO: remove this once the previous version is > 1.29
	unstructured, err := helpers.K0sClusterConfigTo129Compat(&finalcfg)
	if err != nil {
		return fmt.Errorf("unable to convert cluster config to 1.29 compat: %w", err)
	}
	data, err := k8syaml.Marshal(unstructured)
	if err != nil {
		return fmt.Errorf("unable to marshal node config: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("unable to write node config file: %w", err)
	}
	return nil
}

// WaitForK0s waits for the k0s API to be available. We wait for the k0s socket to
// appear in the system and until the k0s status command to finish.
func (k *K0s) WaitForK0s() error {
	var success bool
	for i := 0; i < 30; i++ {
		time.Sleep(2 * time.Second)
		spath := runtimeconfig.K0sStatusSocketPath
		if _, err := os.Stat(spath); err != nil {
			continue
		}
		success = true
		break
	}
	if !success {
		return fmt.Errorf("timeout waiting for %s", runtimeconfig.AppSlug())
	}

	for i := 1; ; i++ {
		_, err := helpers.RunCommand(runtimeconfig.K0sBinaryPath, "status")
		if err == nil {
			return nil
		} else if i == 30 {
			return fmt.Errorf("unable to get status: %w", err)
		}
		time.Sleep(2 * time.Second)
	}
}

func (k *K0s) WaitForAutopilotPlan(ctx context.Context, cli client.Client, logger logrus.FieldLogger) (apv1b2.Plan, error) {
	return k.waitForAutopilotPlanWithBackoff(ctx, cli, logger, wait.Backoff{
		Duration: 20 * time.Second, // 20-second polling interval
		Steps:    90,               // 90 attempts × 20s = 1800s = 30 minutes
	})
}

func (k *K0s) waitForAutopilotPlanWithBackoff(ctx context.Context, cli client.Client, logger logrus.FieldLogger, backoff wait.Backoff) (apv1b2.Plan, error) {
	var plan apv1b2.Plan
	var lastErr error

	err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		err := cli.Get(ctx, client.ObjectKey{Name: "autopilot"}, &plan)
		if err != nil {
			lastErr = fmt.Errorf("get autopilot plan: %w", err)
			return false, nil
		}

		if autopilot.HasThePlanEnded(plan) {
			return true, nil
		}

		logger.WithField("plan_id", plan.Spec.ID).Info("An autopilot upgrade is in progress")
		return false, nil
	})

	if err != nil {
		if errors.Is(err, context.Canceled) {
			if lastErr != nil {
				err = errors.Join(err, lastErr)
			}
			return apv1b2.Plan{}, err
		} else if lastErr != nil {
			return apv1b2.Plan{}, fmt.Errorf("timed out waiting for autopilot plan: %w", lastErr)
		} else {
			return apv1b2.Plan{}, fmt.Errorf("timed out waiting for autopilot plan")
		}
	}

	return plan, nil
}

func (k *K0s) WaitForClusterNodesMatchVersion(ctx context.Context, cli client.Client, desiredVersion string, logger logrus.FieldLogger) error {
	return k.waitForClusterNodesMatchVersionWithBackoff(ctx, cli, desiredVersion, logger, wait.Backoff{
		Duration: 5 * time.Second,
		Steps:    60, // 60 attempts × 5s = 300s = 5 minutes
		Factor:   1.0,
		Jitter:   0.1,
	})
}

func (k *K0s) waitForClusterNodesMatchVersionWithBackoff(ctx context.Context, cli client.Client, desiredVersion string, logger logrus.FieldLogger, backoff wait.Backoff) error {
	var lastErr error

	err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		match, err := k.ClusterNodesMatchVersion(ctx, cli, desiredVersion)
		if err != nil {
			lastErr = fmt.Errorf("check cluster nodes match version: %w", err)
			return false, nil
		}

		if !match {
			logger.WithField("version", desiredVersion).Debug("Waiting for cluster nodes to report updated version")
			return false, nil
		}

		return true, nil
	})

	if err != nil {
		if errors.Is(err, context.Canceled) {
			if lastErr != nil {
				err = errors.Join(err, lastErr)
			}
			return err
		} else if lastErr != nil {
			return fmt.Errorf("timed out waiting for cluster nodes to match version %s: %w", desiredVersion, lastErr)
		} else {
			return fmt.Errorf("cluster nodes did not match version %s after upgrade", desiredVersion)
		}
	}

	return nil
}

// ClusterNodesMatchVersion returns true if all nodes in the cluster have kubeletVersion matching the provided version.
func (k *K0s) ClusterNodesMatchVersion(ctx context.Context, cli client.Client, version string) (bool, error) {
	var nodes corev1.NodeList
	if err := cli.List(ctx, &nodes); err != nil {
		return false, fmt.Errorf("list nodes: %w", err)
	}
	for _, node := range nodes.Items {
		if node.Status.NodeInfo.KubeletVersion != version {
			return false, nil
		}
	}
	return true, nil
}

func (k *K0s) WaitForAirgapArtifactsAutopilotPlan(ctx context.Context, cli client.Client, in *ecv1beta1.Installation) error {
	nsn := types.NamespacedName{Name: "autopilot"}
	plan := apv1b2.Plan{}

	err := wait.PollUntilContextCancel(ctx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		err := cli.Get(ctx, nsn, &plan)
		if err != nil {
			return false, fmt.Errorf("get autopilot plan: %w", err)
		}
		if plan.Annotations[artifacts.InstallationNameAnnotation] != in.Name {
			return false, fmt.Errorf("autopilot plan for different installation")
		}

		switch {
		case autopilot.HasPlanSucceeded(plan):
			return true, nil
		case autopilot.HasPlanFailed(plan):
			reason := autopilot.ReasonForState(plan)
			return false, fmt.Errorf("autopilot plan failed: %s", reason)
		}
		// plan is still running
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("wait for autopilot plan: %w", err)
	}

	return nil
}
