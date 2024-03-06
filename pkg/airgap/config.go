package airgap

import "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

func SetAirgapConfig(cfg *v1beta1.ClusterConfig) {
	if cfg.Spec.Images == nil {
		cfg.Spec.Images = &v1beta1.ClusterImages{}
	}

	cfg.Spec.Images.DefaultPullPolicy = "Never"
}
