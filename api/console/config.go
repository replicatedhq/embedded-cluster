package console

import (
	"sync"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
)

type Config struct {
	AdminConsolePassword    string `json:"adminConsolePassword"`
	AdminConsolePort        int    `json:"adminConsolePort"`
	DataDirectory           string `json:"dataDirectory"`
	LocalArtifactMirrorPort int    `json:"localArtifactMirrorPort"`
	NetworkInterface        string `json:"networkInterface"`
	HTTPProxy               string `json:"httpProxy"`
	HTTPSProxy              string `json:"httpsProxy"`
	NoProxy                 string `json:"noProxy"`
	PodCIDR                 string `json:"podCIDR"`
	ServiceCIDR             string `json:"serviceCIDR"`
	GlobalCIDR              string `json:"globalCIDR"`
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
