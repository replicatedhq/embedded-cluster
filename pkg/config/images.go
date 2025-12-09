package config

import (
	"strings"

	"github.com/k0sproject/k0s/pkg/airgap"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
)

func ListK0sImages(cfg *k0sv1beta1.ClusterConfig) []string {
	var images []string
	for _, image := range airgap.GetImageURIs(cfg.Spec, true) {
		switch image {
		// skip these images
		case cfg.Spec.Images.KubeRouter.CNI.URI(),
			cfg.Spec.Images.KubeRouter.CNIInstaller.URI(),
			cfg.Spec.Images.Konnectivity.URI(),
			cfg.Spec.Images.PushGateway.URI():
		default:
			images = append(images, image)
		}
	}
	return images
}

func overrideK0sImages(cfg *k0sv1beta1.ClusterConfig, proxyRegistryDomain string) {
	if _metadata == nil {
		panic("k0s version is not set")
	}

	if cfg.Spec.Images == nil {
		cfg.Spec.Images = &k0sv1beta1.ClusterImages{}
	}
	if cfg.Spec.Network == nil {
		cfg.Spec.Network = &k0sv1beta1.Network{}
	}
	if cfg.Spec.Network.NodeLocalLoadBalancing == nil {
		cfg.Spec.Network.NodeLocalLoadBalancing = &k0sv1beta1.NodeLocalLoadBalancing{}
	}
	if cfg.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy == nil {
		cfg.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy = &k0sv1beta1.EnvoyProxy{}
	}

	if proxyRegistryDomain == "" {
		cfg.Spec.Images.CoreDNS.Image = _metadata.Images["coredns"].Repo
		cfg.Spec.Images.Calico.Node.Image = _metadata.Images["calico-node"].Repo
		cfg.Spec.Images.Calico.CNI.Image = _metadata.Images["calico-cni"].Repo
		cfg.Spec.Images.Calico.KubeControllers.Image = _metadata.Images["calico-kube-controllers"].Repo
		cfg.Spec.Images.MetricsServer.Image = _metadata.Images["metrics-server"].Repo
		cfg.Spec.Images.KubeProxy.Image = _metadata.Images["kube-proxy"].Repo
		cfg.Spec.Images.Pause.Image = _metadata.Images["pause"].Repo
		cfg.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy.Image.Image = _metadata.Images["envoy-distroless"].Repo
	} else {
		cfg.Spec.Images.CoreDNS.Image = strings.Replace(_metadata.Images["coredns"].Repo, "proxy.replicated.com", proxyRegistryDomain, 1)
		cfg.Spec.Images.Calico.Node.Image = strings.Replace(_metadata.Images["calico-node"].Repo, "proxy.replicated.com", proxyRegistryDomain, 1)
		cfg.Spec.Images.Calico.CNI.Image = strings.Replace(_metadata.Images["calico-cni"].Repo, "proxy.replicated.com", proxyRegistryDomain, 1)
		cfg.Spec.Images.Calico.KubeControllers.Image = strings.Replace(_metadata.Images["calico-kube-controllers"].Repo, "proxy.replicated.com", proxyRegistryDomain, 1)
		cfg.Spec.Images.MetricsServer.Image = strings.Replace(_metadata.Images["metrics-server"].Repo, "proxy.replicated.com", proxyRegistryDomain, 1)
		cfg.Spec.Images.KubeProxy.Image = strings.Replace(_metadata.Images["kube-proxy"].Repo, "proxy.replicated.com", proxyRegistryDomain, 1)
		cfg.Spec.Images.Pause.Image = strings.Replace(_metadata.Images["pause"].Repo, "proxy.replicated.com", proxyRegistryDomain, 1)
		cfg.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy.Image.Image = strings.Replace(_metadata.Images["envoy-distroless"].Repo, "proxy.replicated.com", proxyRegistryDomain, 1)
	}

	cfg.Spec.Images.CoreDNS.Version = _metadata.Images["coredns"].Tag[helpers.ClusterArch()]
	cfg.Spec.Images.Calico.Node.Version = _metadata.Images["calico-node"].Tag[helpers.ClusterArch()]
	cfg.Spec.Images.Calico.CNI.Version = _metadata.Images["calico-cni"].Tag[helpers.ClusterArch()]
	cfg.Spec.Images.Calico.KubeControllers.Version = _metadata.Images["calico-kube-controllers"].Tag[helpers.ClusterArch()]
	cfg.Spec.Images.MetricsServer.Version = _metadata.Images["metrics-server"].Tag[helpers.ClusterArch()]
	cfg.Spec.Images.KubeProxy.Version = _metadata.Images["kube-proxy"].Tag[helpers.ClusterArch()]
	cfg.Spec.Images.Pause.Version = _metadata.Images["pause"].Tag[helpers.ClusterArch()]
	cfg.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy.Image.Version = _metadata.Images["envoy-distroless"].Tag[helpers.ClusterArch()]
}
