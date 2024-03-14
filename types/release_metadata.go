package types

import "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

// ReleaseMetadata holds the metadata about a specific release, including addons and
// their versions.
type ReleaseMetadata struct {
	Versions      map[string]string
	K0sSHA        string
	K0sBinaryURL  string
	K0sImages     []string
	Configs       v1beta1.HelmExtensions
	AirgapConfigs v1beta1.HelmExtensions
	Protected     map[string][]string
}
