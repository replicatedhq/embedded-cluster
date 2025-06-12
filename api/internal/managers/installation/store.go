package installation

import (
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

// TODO (@team): discuss the idea of having a generic store interface that can be used for all stores
type InstallationStore interface {
	GetConfig() (*types.InstallationConfig, error)
	SetConfig(cfg types.InstallationConfig) error
	GetStatus() (*types.Status, error)
	SetStatus(status types.Status) error
}

var _ InstallationStore = &MemoryStore{}

type MemoryStore struct {
	mu       sync.RWMutex
	rc       runtimeconfig.RuntimeConfig
	inStatus *types.Status
}

func NewMemoryStore(rc runtimeconfig.RuntimeConfig, inStatus *types.Status) *MemoryStore {
	return &MemoryStore{
		rc:       rc,
		inStatus: inStatus,
	}
}

func (s *MemoryStore) GetConfig() (*types.InstallationConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.newInstallationConfigFromRC(s.rc), nil
}

func (s *MemoryStore) SetConfig(cfg types.InstallationConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.setRCFromInstallationConfig(cfg)

	return nil
}

func (s *MemoryStore) GetStatus() (*types.Status, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.inStatus, nil
}

func (s *MemoryStore) SetStatus(status types.Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inStatus = &status

	return nil
}

func (s *MemoryStore) newInstallationConfigFromRC(rc runtimeconfig.RuntimeConfig) *types.InstallationConfig {
	config := &types.InstallationConfig{
		DataDirectory:           rc.EmbeddedClusterHomeDirectory(),
		AdminConsolePort:        rc.AdminConsolePort(),
		LocalArtifactMirrorPort: rc.LocalArtifactMirrorPort(),
		NetworkInterface:        rc.NetworkInterface(),
		GlobalCIDR:              rc.GlobalCIDR(),
		PodCIDR:                 rc.PodCIDR(),
		ServiceCIDR:             rc.ServiceCIDR(),
	}

	if rc.ProxySpec() != nil {
		config.HTTPProxy = rc.ProxySpec().HTTPProxy
		config.HTTPSProxy = rc.ProxySpec().HTTPSProxy
		config.NoProxy = rc.ProxySpec().NoProxy
	}
	return config
}

func (s *MemoryStore) setRCFromInstallationConfig(cfg types.InstallationConfig) {
	s.rc.SetDataDir(cfg.DataDirectory)
	s.rc.SetAdminConsolePort(cfg.AdminConsolePort)
	s.rc.SetLocalArtifactMirrorPort(cfg.LocalArtifactMirrorPort)

	s.rc.SetNetworkSpec(ecv1beta1.NetworkSpec{
		NetworkInterface: cfg.NetworkInterface,
		GlobalCIDR:       cfg.GlobalCIDR,
		PodCIDR:          cfg.PodCIDR,
		ServiceCIDR:      cfg.ServiceCIDR,
	})

	var proxySpec *ecv1beta1.ProxySpec
	if cfg.HTTPProxy != "" || cfg.HTTPSProxy != "" || cfg.NoProxy != "" {
		proxySpec = &ecv1beta1.ProxySpec{
			HTTPProxy:  cfg.HTTPProxy,
			HTTPSProxy: cfg.HTTPSProxy,
			NoProxy:    cfg.NoProxy,
		}
	}
	s.rc.SetProxySpec(proxySpec)
}
