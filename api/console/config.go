package console

import (
	"errors"
	"fmt"
	"sync"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

type Config struct {
	AdminConsolePassword    string `json:"adminConsolePassword"`
	AdminConsolePort        int    `json:"adminConsolePort"`
	DataDirectory           string `json:"dataDirectory"`
	HostCABundlePath        string `json:"hostCABundlePath"`
	LocalArtifactMirrorPort int    `json:"localArtifactMirrorPort"`
	NetworkInterface        string `json:"networkInterface"`
	HTTPProxy               string `json:"httpProxy"`
	HTTPSProxy              string `json:"httpsProxy"`
	NoProxy                 string `json:"noProxy"`
	PodCIDR                 string `json:"podCidr"`
	ServiceCIDR             string `json:"serviceCidr"`
	GlobalCIDR              string `json:"globalCidr"`
	Overrides               string `json:"overrides"`
}

func (c *Config) GetProxySpec() *ecv1beta1.ProxySpec {
	if c.HTTPProxy == "" && c.HTTPSProxy == "" && c.NoProxy == "" {
		return nil
	}
	return &ecv1beta1.ProxySpec{
		HTTPProxy:  c.HTTPProxy,
		HTTPSProxy: c.HTTPSProxy,
		NoProxy:    c.NoProxy,
	}
}

type configStore interface {
	read() (*Config, error)
	write(cfg *Config) error
}

var _ configStore = &configMemoryStore{}

type configMemoryStore struct {
	mu  sync.RWMutex
	cfg *Config
}

func (s *configMemoryStore) read() (*Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.cfg, nil
}

func (s *configMemoryStore) write(cfg *Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg

	return nil
}

func validateConfig(config Config) error {
	if config.AdminConsolePassword == "" {
		return errors.New("adminConsolePassword is required")
	}

	if err := validateConfigCIDR(config); err != nil {
		return err
	}

	if err := validateConfigNetworkInterface(config); err != nil {
		return err
	}

	if err := validateConfigPorts(config); err != nil {
		return err
	}

	return nil
}

func validateConfigCIDR(config Config) error {
	if config.PodCIDR != "" && config.ServiceCIDR == "" {
		return errors.New("serviceCidr is required when podCidr is set")
	}

	if config.ServiceCIDR != "" && config.PodCIDR == "" {
		return errors.New("podCidr is required when serviceCidr is set")
	}

	if config.GlobalCIDR != "" {
		if config.PodCIDR != "" || config.ServiceCIDR != "" {
			podCIDR, serviceCIDR, err := netutils.SplitNetworkCIDR(config.GlobalCIDR)
			if err != nil {
				return fmt.Errorf("globalCidr: %w", err)
			}

			if podCIDR != config.PodCIDR {
				return fmt.Errorf("podCidr does not match globalCIDR")
			}

			if serviceCIDR != config.ServiceCIDR {
				return fmt.Errorf("serviceCidr does not match globalCIDR")
			}
		}

		if err := netutils.ValidateCIDR(config.GlobalCIDR, 16, true); err != nil {
			return fmt.Errorf("globalCidr: %w", err)
		}
	}

	return nil
}

func validateConfigNetworkInterface(config Config) error {
	if config.NetworkInterface == "" {
		return nil
	}

	// TODO: validate the network interface exists and is up and not loopback

	return nil
}

func validateConfigPorts(config Config) error {
	lamPort := config.LocalArtifactMirrorPort
	acPort := config.AdminConsolePort

	if lamPort != 0 && acPort != 0 {
		if lamPort == acPort {
			return fmt.Errorf("localArtifactMirrorPort and adminConsolePort cannot be equal")
		}
	}

	return nil
}

func configSetDefaults(config *Config) error {
	if config.AdminConsolePort == 0 {
		config.AdminConsolePort = ecv1beta1.DefaultAdminConsolePort
	}

	if config.DataDirectory == "" {
		config.DataDirectory = ecv1beta1.DefaultDataDir
	}

	// if a host CA bundle path was not provided, attempt to discover it
	if config.HostCABundlePath == "" {
		hostCABundlePath, err := findHostCABundle()
		if err != nil {
			return fmt.Errorf("unable to find host CA bundle: %w", err)
		}
		config.HostCABundlePath = hostCABundlePath
	}

	if config.LocalArtifactMirrorPort == 0 {
		config.LocalArtifactMirrorPort = ecv1beta1.DefaultLocalArtifactMirrorPort
	}

	// if a network interface was not provided, attempt to discover it
	if config.NetworkInterface == "" {
		autoInterface, err := netutils.DetermineBestNetworkInterface()
		if err == nil {
			config.NetworkInterface = autoInterface
		}
	}

	if config.GlobalCIDR == "" && config.PodCIDR == "" && config.ServiceCIDR == "" {
		config.GlobalCIDR = ecv1beta1.DefaultNetworkCIDR
	}

	// if podCIDR or serviceCIDR is not set, we need to split the globalCIDR
	if config.PodCIDR == "" || config.ServiceCIDR == "" {
		podCIDR, serviceCIDR, err := netutils.SplitNetworkCIDR(config.GlobalCIDR)
		if err != nil {
			return fmt.Errorf("unable to split network cidr: %w", err)
		}
		config.PodCIDR = podCIDR
		config.ServiceCIDR = serviceCIDR
	}

	return nil
}

func applyConfigToRuntimeConfig(config Config) error {
	if config.DataDirectory != "" {
		runtimeconfig.SetDataDir(config.DataDirectory)
	}

	if config.HostCABundlePath != "" {
		runtimeconfig.SetHostCABundlePath(config.HostCABundlePath)
	}

	if config.LocalArtifactMirrorPort != 0 {
		runtimeconfig.SetLocalArtifactMirrorPort(config.LocalArtifactMirrorPort)
	}

	if config.AdminConsolePort != 0 {
		runtimeconfig.SetAdminConsolePort(config.AdminConsolePort)
	}

	return nil
}
