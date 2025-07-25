package types

import (
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

// APIConfig holds the configuration for the API server
type APIConfig struct {
	RuntimeConfig             runtimeconfig.RuntimeConfig
	Password                  string
	TLSConfig                 TLSConfig
	License                   []byte
	AirgapBundle              string
	AirgapMetadata            *airgap.AirgapMetadata
	EmbeddedAssetsSize        int64
	ConfigValues              string
	ReleaseData               *release.ReleaseData
	EndUserConfig             *ecv1beta1.Config
	AllowIgnoreHostPreflights bool
}
