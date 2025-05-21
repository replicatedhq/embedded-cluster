package console

import (
	"errors"
	"fmt"
	"os"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
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

func (c *Config) InitEnvironment(logger logrus.FieldLogger) error {
	if err := applyConfigToRuntimeConfig(*c); err != nil {
		return fmt.Errorf("apply config to runtime config: %w", err)
	}

	proxySpec, err := c.GetProxySpec()
	if err != nil {
		return fmt.Errorf("get proxy spec: %w", err)
	}

	if err := runtimeconfig.WriteToDisk(); err != nil {
		return fmt.Errorf("write runtime config to disk: %w", err)
	}

	os.Setenv("KUBECONFIG", runtimeconfig.PathToKubeConfig())
	os.Setenv("TMPDIR", runtimeconfig.EmbeddedClusterTmpSubDir())

	if proxySpec != nil {
		if proxySpec.HTTPProxy != "" {
			os.Setenv("HTTP_PROXY", proxySpec.HTTPProxy)
		}
		if proxySpec.HTTPSProxy != "" {
			os.Setenv("HTTPS_PROXY", proxySpec.HTTPSProxy)
		}
		if proxySpec.NoProxy != "" {
			os.Setenv("NO_PROXY", proxySpec.NoProxy)
		}
	}

	if err := os.Chmod(runtimeconfig.EmbeddedClusterHomeDirectory(), 0755); err != nil {
		// don't fail as there are cases where we can't change the permissions (bind mounts, selinux, etc...),
		// and we handle and surface those errors to the user later (host preflights, checking exec errors, etc...)
		logger.Debugf("unable to chmod embedded-cluster home dir: %s", err)
	}

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

func (c *Config) GetProxySpec() (*ecv1beta1.ProxySpec, error) {
	if c.HTTPProxy == "" && c.HTTPSProxy == "" && c.NoProxy == "" {
		return nil, nil
	}

	proxySpec := ecv1beta1.ProxySpec{
		HTTPProxy:       c.HTTPProxy,
		HTTPSProxy:      c.HTTPSProxy,
		ProvidedNoProxy: c.NoProxy,
	}

	// Now that we have all no-proxy entries (from flags/env), merge in defaults
	noProxy, err := combineNoProxySuppliedValuesAndDefaults(*c, proxySpec, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to combine no-proxy supplied values and defaults: %w", err)
	}
	proxySpec.NoProxy = noProxy

	return &proxySpec, nil
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

func configSetDefaults(logger logrus.FieldLogger, config *Config) error {
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

	if err := configSetCIDRDefaults(config); err != nil {
		return fmt.Errorf("unable to set cidr defaults: %w", err)
	}

	configSetProxyDefaults(logger, config)

	return nil
}

func configSetProxyDefaults(logger logrus.FieldLogger, config *Config) {
	if config.HTTPProxy == "" {
		if envValue := os.Getenv("http_proxy"); envValue != "" {
			logger.Debug("got http_proxy from http_proxy env var")
			config.HTTPProxy = envValue
		} else if envValue := os.Getenv("HTTP_PROXY"); envValue != "" {
			logger.Debug("got http_proxy from HTTP_PROXY env var")
			config.HTTPProxy = envValue
		}
	}
	if config.HTTPSProxy == "" {
		if envValue := os.Getenv("https_proxy"); envValue != "" {
			logger.Debug("got https_proxy from https_proxy env var")
			config.HTTPSProxy = envValue
		} else if envValue := os.Getenv("HTTPS_PROXY"); envValue != "" {
			logger.Debug("got https_proxy from HTTPS_PROXY env var")
			config.HTTPSProxy = envValue
		}
	}
	if config.NoProxy == "" {
		if envValue := os.Getenv("no_proxy"); envValue != "" {
			logger.Debug("got no_proxy from no_proxy env var")
			config.NoProxy = envValue
		} else if envValue := os.Getenv("NO_PROXY"); envValue != "" {
			logger.Debug("got no_proxy from NO_PROXY env var")
			config.NoProxy = envValue
		}
	}
}

func configSetCIDRDefaults(config *Config) error {
	if config.PodCIDR == "" && config.ServiceCIDR == "" {
		if config.GlobalCIDR == "" {
			config.GlobalCIDR = ecv1beta1.DefaultNetworkCIDR
		}

		podCIDR, serviceCIDR, err := netutils.SplitNetworkCIDR(config.GlobalCIDR)
		if err != nil {
			return fmt.Errorf("split network cidr: %w", err)
		}
		config.PodCIDR = podCIDR
		config.ServiceCIDR = serviceCIDR

		return nil
	}

	if config.PodCIDR == "" {
		config.PodCIDR = k0sv1beta1.DefaultNetwork().PodCIDR
	}

	if config.ServiceCIDR == "" {
		config.ServiceCIDR = k0sv1beta1.DefaultNetwork().ServiceCIDR
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
