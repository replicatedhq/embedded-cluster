package kotsadm

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/k0sproject/dig"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"gopkg.in/yaml.v2"
)

// JoinCommandResponse is the response from the kots api we use to fetch the k0s join token.
type JoinCommandResponse struct {
	K0sJoinCommand         string                     `json:"k0sJoinCommand"`
	K0sToken               string                     `json:"k0sToken"`
	ClusterID              uuid.UUID                  `json:"clusterID"`
	EmbeddedClusterVersion string                     `json:"embeddedClusterVersion"`
	AirgapRegistryAddress  string                     `json:"airgapRegistryAddress"`
	TCPConnectionsRequired []string                   `json:"tcpConnectionsRequired"`
	InstallationSpec       ecv1beta1.InstallationSpec `json:"installationSpec,omitempty"`
}

// extractK0sConfigOverridePatch parses the provided override and returns a dig.Mapping that
// can be then applied on top a k0s configuration file to set both `api` and `storage` spec
// fields. All other fields in the override are ignored.
func (j JoinCommandResponse) extractK0sConfigOverridePatch(data []byte) (dig.Mapping, error) {
	config := dig.Mapping{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unable to unmarshal embedded config: %w", err)
	}
	fmt.Println(config)
	fmt.Println("--------------------------------")
	result := dig.Mapping{}
	if api := config.DigMapping("config", "spec", "api"); len(api) > 0 {
		result.DigMapping("config", "spec")["api"] = api
	}
	if storage := config.DigMapping("config", "spec", "storage"); len(storage) > 0 {
		result.DigMapping("config", "spec")["storage"] = storage
	}
	if workerProfiles := config.DigMapping("config", "spec", "workerProfiles"); len(workerProfiles) > 0 {
		result.DigMapping("config", "spec")["workerProfiles"] = workerProfiles
	}
	fmt.Println(result)
	fmt.Println("--------------------------------")
	return result, nil
}

// EndUserOverrides returns a dig.Mapping that can be applied on top of a k0s configuration.
// This patch is assembled based on the EndUserK0sConfigOverrides field.
func (j JoinCommandResponse) EndUserOverrides() (dig.Mapping, error) {
	return j.extractK0sConfigOverridePatch([]byte(j.InstallationSpec.EndUserK0sConfigOverrides))
}

// EmbeddedOverrides returns a dig.Mapping that can be applied on top of a k0s configuration.
// This patch is assembled based on the K0sUnsupportedOverrides field.
func (j JoinCommandResponse) EmbeddedOverrides() (dig.Mapping, error) {
	return j.extractK0sConfigOverridePatch([]byte(j.InstallationSpec.Config.UnsupportedOverrides.K0s))
}
