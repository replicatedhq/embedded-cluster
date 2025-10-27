package k0s

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/domains"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8syaml "sigs.k8s.io/yaml"
)

const (
	k0sBinPath = "/usr/local/bin/k0s"
)

var _ K0sInterface = (*K0s)(nil)

type K0s struct {
}

func New() *K0s {
	return &K0s{}
}

// GetStatus calls the k0s status command and returns information about system init, PID, k0s role,
// kubeconfig and similar.
func (k *K0s) GetStatus(ctx context.Context) (*K0sStatus, error) {
	if _, err := helpers.Stat(k0sBinPath); err != nil {
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
	_, err := helpers.Stat(runtimeconfig.K0sConfigPath)
	if err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	}

	return false, fmt.Errorf("unable to check if already installed: %w", err)
}

// NewK0sConfig creates a new k0sv1beta1.ClusterConfig object from the input parameters.
func NewK0sConfig(networkInterface string, isAirgap bool, podCIDR string, serviceCIDR string, eucfg *ecv1beta1.Config, mutate func(*k0sv1beta1.ClusterConfig) error) (*k0sv1beta1.ClusterConfig, error) {
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
func (k *K0s) WriteK0sConfig(ctx context.Context, networkInterface string, airgapBundle string, podCIDR string, serviceCIDR string, eucfg *ecv1beta1.Config, mutate func(*k0sv1beta1.ClusterConfig) error) (*k0sv1beta1.ClusterConfig, error) {
	cfg, err := NewK0sConfig(networkInterface, airgapBundle != "", podCIDR, serviceCIDR, eucfg, mutate)
	if err != nil {
		return nil, fmt.Errorf("unable to create k0s config: %w", err)
	}

	cfgpath := runtimeconfig.K0sConfigPath
	if _, err := helpers.Stat(cfgpath); err == nil {
		return nil, fmt.Errorf("configuration file already exists")
	}
	if err := helpers.MkdirAll(filepath.Dir(cfgpath), 0755); err != nil {
		return nil, fmt.Errorf("unable to create directory: %w", err)
	}

	// This is necessary to install the previous version of k0s in e2e tests
	// TODO: remove this once the previous version is > 1.29
	unstructured, err := helpers.K0sClusterConfigTo129Compat(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to convert cluster config to 1.29 compat: %w", err)
	}
	data, err := k8syaml.Marshal(unstructured)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal config: %w", err)
	}
	if err := helpers.WriteFile(cfgpath, data, 0600); err != nil {
		return nil, fmt.Errorf("unable to write config file: %w", err)
	}

	return cfg, nil
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
	if _, err := helpers.Stat(path); err == nil {
		data, err := helpers.ReadFile(path)
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
	if err := helpers.WriteFile(path, data, 0600); err != nil {
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
		if _, err := helpers.Stat(spath); err != nil {
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
