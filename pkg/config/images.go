package config

import (
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/config/images"
)

func OverrideK0sImages(cfg *k0sv1beta1.ClusterConfig) error {
	if cfg.Spec.Images == nil {
		cfg.Spec.Images = &k0sv1beta1.ClusterImages{}
	}
	if images.CoreDNSImage != "" {
		cfg.Spec.Images.CoreDNS.Image = images.CoreDNSImage
	}
	if images.CoreDNSVersion != "" {
		cfg.Spec.Images.CoreDNS.Version = images.CoreDNSVersion
	}
	if images.CalicoNodeImage != "" {
		cfg.Spec.Images.Calico.Node.Image = images.CalicoNodeImage
	}
	if images.CalicoNodeVersion != "" {
		cfg.Spec.Images.Calico.Node.Version = images.CalicoNodeVersion
	}
	if images.CalicoCNIImage != "" {
		cfg.Spec.Images.Calico.CNI.Image = images.CalicoCNIImage
	}
	if images.CalicoCNIVersion != "" {
		cfg.Spec.Images.Calico.CNI.Version = images.CalicoCNIVersion
	}
	if images.CalicoKubeControllersImage != "" {
		cfg.Spec.Images.Calico.KubeControllers.Image = images.CalicoKubeControllersImage
	}
	if images.CalicoKubeControllersVersion != "" {
		cfg.Spec.Images.Calico.KubeControllers.Version = images.CalicoKubeControllersVersion
	}
	if images.MetricsServerImage != "" {
		cfg.Spec.Images.MetricsServer.Image = images.MetricsServerImage
	}
	if images.MetricsServerVersion != "" {
		cfg.Spec.Images.MetricsServer.Version = images.MetricsServerVersion
	}
	if images.KubeProxyImage != "" {
		cfg.Spec.Images.KubeProxy.Image = images.KubeProxyImage
	}
	if images.KubeProxyVersion != "" {
		cfg.Spec.Images.KubeProxy.Version = images.KubeProxyVersion
	}
	return nil
}
