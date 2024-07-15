package config

import (
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/config/images"
)

func OverrideK0sImages(cfg *k0sv1beta1.ClusterConfig) error {
	if cfg.Spec.Images == nil {
		cfg.Spec.Images = &k0sv1beta1.ClusterImages{}
	}
	if images.CalicoNodeImage != "" {
		cfg.Spec.Images.Calico.Node.Image = images.CalicoNodeImage
	}
	if images.CalicoNodeVersion != "" {
		cfg.Spec.Images.Calico.Node.Version = images.CalicoNodeVersion
	}
	return nil
}
