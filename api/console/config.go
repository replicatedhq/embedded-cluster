package console

import "sync"

type Config struct {
	AdminConsolePassword    string `json:"adminConsolePassword"`
	AdminConsolePort        int    `json:"adminConsolePort"`
	DataDirectory           string `json:"dataDirectory"`
	LocalArtifactMirrorPort int    `json:"localArtifactMirrorPort"`
	NetworkInterface        string `json:"networkInterface"`
	HTTPProxy               string `json:"httpProxy"`
	HTTPSProxy              string `json:"httpsProxy"`
	NoProxy                 string `json:"noProxy"`
	GlobalCIDR              string `json:"globalCIDR"`
	PodCIDR                 string `json:"podCIDR"`
	ServiceCIDR             string `json:"serviceCIDR"`
	Overrides               string `json:"overrides"`
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
