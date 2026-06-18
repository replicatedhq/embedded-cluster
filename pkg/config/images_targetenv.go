//go:build !k0s_legacy_airgap

package config

import (
	"github.com/k0sproject/k0s/pkg/airgap"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	imagespecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// allK0sImageURIs returns the full set of k0s image URIs for k0s 1.36+, where
// airgap.GetImageURIs takes an airgap.TargetEnv. The platform OS must be set to
// "linux" or GetImageURIs filters out the base images.
func allK0sImageURIs(cfg *k0sv1beta1.ClusterConfig) []string {
	env := airgap.TargetEnv{
		Platform: imagespecv1.Platform{OS: "linux"},
		Spec:     cfg.Spec,
	}
	return airgap.GetImageURIs(env, true)
}
