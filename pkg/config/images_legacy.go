//go:build k0s_legacy_airgap

package config

import (
	"github.com/k0sproject/k0s/pkg/airgap"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

// allK0sImageURIs returns the full set of k0s image URIs for k0s <= 1.35, where
// airgap.GetImageURIs takes a bare *ClusterSpec. Selected by the
// k0s_legacy_airgap build tag (see images.go).
// TODO(k0s-1.36-oldest): drop this file and the build tag.
func allK0sImageURIs(cfg *k0sv1beta1.ClusterConfig) []string {
	return airgap.GetImageURIs(cfg.Spec, true)
}
