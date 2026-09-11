package helpers

import (
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
)

func NetworkSpecFromK0sConfig(k0sCfg *k0sv1beta1.ClusterConfig) ecv1beta1.NetworkSpec {
	network := ecv1beta1.NetworkSpec{}

	if k0sCfg.Spec != nil && k0sCfg.Spec.Network != nil {
		network.PodCIDR = k0sCfg.Spec.Network.PodCIDR
		network.ServiceCIDR = k0sCfg.Spec.Network.ServiceCIDR
	}

	if k0sCfg.Spec.API != nil {
		if val, ok := k0sCfg.Spec.API.ExtraArgs["service-node-port-range"]; ok {
			network.NodePortRange = val
		}
	}

	return network
}

// This code was copied from the k0s project to maintain backwards compatibility with versions < 1.33
// https://github.com/k0sproject/k0s/blob/4615902bc8c4fbbb8f150371f8f60818458479c9/pkg/apis/k0s/v1beta1/clusterconfig_types.go#L264-L278
func K0sConfigFromBytes(yml []byte) (*k0sv1beta1.ClusterConfig, error) {
	c := k0sv1beta1.DefaultClusterConfig()
	merged := c.DeepCopy()
	err := YamlUnmarshalStrictIgnoringFields(yml, merged, "interval", "podSecurityPolicy")
	if err != nil {
		return nil, err
	}
	if merged.Spec == nil {
		merged.Spec = c.Spec
	}
	return merged, nil
}
