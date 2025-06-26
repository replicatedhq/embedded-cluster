package types

import (
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

// APIConfig holds the configuration for the API server
type APIConfig struct {
	RuntimeConfig             runtimeconfig.RuntimeConfig
	Password                  string
	TLSConfig                 TLSConfig
	License                   []byte
	AirgapBundle              string
	AirgapInfo                *kotsv1beta1.Airgap
	ConfigValues              string
	ReleaseData               *release.ReleaseData
	EndUserConfig             *ecv1beta1.Config
	AllowIgnoreHostPreflights bool
}
