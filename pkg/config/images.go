package config

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/k0sproject/k0s/pkg/airgap"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"gopkg.in/yaml.v3"
)

var (
	//go:embed static/metadata-1_31.yaml
	_rawmetadata1_31 []byte
	//go:embed static/metadata-1_30.yaml
	_rawmetadata1_30 []byte
	//go:embed static/metadata-1_29.yaml
	_rawmetadata1_29 []byte

	_metadata1_31 release.K0sMetadata
	_metadata1_30 release.K0sMetadata
	_metadata1_29 release.K0sMetadata

	_metadata *release.K0sMetadata
)

func init() {
	if err := yaml.Unmarshal(_rawmetadata1_31, &_metadata1_31); err != nil {
		panic(fmt.Sprintf("unable to unmarshal metadata1_31: %v", err))
	}
	if err := yaml.Unmarshal(_rawmetadata1_30, &_metadata1_30); err != nil {
		panic(fmt.Sprintf("unable to unmarshal metadata1_30: %v", err))
	}
	if err := yaml.Unmarshal(_rawmetadata1_29, &_metadata1_29); err != nil {
		panic(fmt.Sprintf("unable to unmarshal metadata1_29: %v", err))
	}

	if versions.K0sVersion != "0.0.0" {
		m := Metadata(versions.K0sVersion)
		_metadata = &m
	}
}

func Metadata(ver string) release.K0sMetadata {
	sv, err := semver.NewVersion(ver)
	if err != nil {
		panic(fmt.Sprintf("unable to parse k0s version %s: %v", ver, err))
	}

	switch fmt.Sprintf("%d.%d", sv.Major(), sv.Minor()) {
	case "1.31":
		return _metadata1_31
	case "1.30":
		return _metadata1_30
	case "1.29":
		return _metadata1_29
	default:
		panic(fmt.Sprintf("no metadata found for k0s version: %s", ver))
	}
}

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
			if strings.Contains(image, constant.KubePauseContainerImage) {
				// there's a bug in GetImageURIs where it always returns the original pause image
				// instead of the one in the config, make sure to use the one in the config.
				// This has been fixed in k0s 1.31, so we can drop it once we drop support for older k0s versions
				// https://github.com/k0sproject/k0s/pull/5520
				images = append(images, cfg.Spec.Images.Pause.URI())
			} else {
				images = append(images, image)
			}
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
