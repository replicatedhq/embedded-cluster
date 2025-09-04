package join

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/k0sproject/dig"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"go.yaml.in/yaml/v3"
)

// JoinCommandResponse is the response from the kots api we use to fetch the k0s join token.
type JoinCommandResponse struct {
	K0sJoinCommand         string                     `json:"k0sJoinCommand"`
	K0sToken               string                     `json:"k0sToken"`
	ClusterID              uuid.UUID                  `json:"clusterID"`
	EmbeddedClusterVersion string                     `json:"embeddedClusterVersion"`
	AppVersionLabel        string                     `json:"appVersionLabel"`
	AirgapRegistryAddress  string                     `json:"airgapRegistryAddress"`
	TCPConnectionsRequired []string                   `json:"tcpConnectionsRequired"`
	InstallationSpec       ecv1beta1.InstallationSpec `json:"installationSpec,omitempty"`
}

// extractK0sConfigOverridePatch parses the provided override and returns a dig.Mapping that
// can be then applied on top a k0s configuration file to set `api`, `storage` and `workerProfiles` spec
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
	workerProfiles := config.Dig("config", "spec", "workerProfiles")
	if workerProfiles != nil {
		result.DigMapping("config", "spec")["workerProfiles"] = workerProfiles
	}
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
