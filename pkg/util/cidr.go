package util

import (
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	v1beta12 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
)

// ClusterServiceCIDR determines the service CIDR for the cluster
// if there is no custom service CIDR, return the default
// if the service CIDR is set in the cluster config, use that - unless
// if the service CIDR is set in the installation, use that because occasionally the cluster config is incorrect
func ClusterServiceCIDR(clusterConfig v1beta1.ClusterConfig, in *v1beta12.Installation) string {
	serviceCIDR := v1beta1.DefaultNetwork().ServiceCIDR
	if clusterConfig.Spec != nil && clusterConfig.Spec.Network != nil {
		serviceCIDR = clusterConfig.Spec.Network.ServiceCIDR
	}
	if in.Spec.Network != nil && in.Spec.Network.ServiceCIDR != "" {
		serviceCIDR = in.Spec.Network.ServiceCIDR
	}
	return serviceCIDR
}
