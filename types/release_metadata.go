package types

import "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

// ReleaseMetadata holds the metadata about a specific release, including addons and
// their versions.
type ReleaseMetadata struct {
	Versions       map[string]string
	K0sSHA         string
	K0sBinaryURL   string
	BinaryURLs     map[string]string // key is the binary name, value is the URL
	K0sImages      []string
	Configs        v1beta1.HelmExtensions            // always applied
	BuiltinConfigs map[string]v1beta1.HelmExtensions // applied if the relevant builtin addon is enabled
	Protected      map[string][]string
}
