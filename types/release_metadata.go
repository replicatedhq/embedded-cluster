package types

import "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

// ReleaseMetadata holds the metadata about a specific release, including addons and
// their versions.
type ReleaseMetadata struct {
	Versions       map[string]string
	K0sSHA         string
	K0sBinaryURL   string
	Artifacts      map[string]string // key is the artifact name, value is the URL it can be retrieved from
	K0sImages      []string
	Configs        v1beta1.HelmExtensions            // always applied
	BuiltinConfigs map[string]v1beta1.HelmExtensions // applied if the relevant builtin addon is enabled
	Protected      map[string][]string

	// Deprecated: AirgapConfigs exists for historical compatibility and should not
	// be used. This field has been replaced by the BuiltinConfigs field.
	AirgapConfigs v1beta1.HelmExtensions
}
