package k8sutil

import (
	clusterv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
)

func ApplyCommonLabels(labels map[string]string, in *clusterv1beta1.Installation, component string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	if in != nil {
		labels["app.kubernetes.io/name"] = in.Spec.BinaryName
		labels["app.kubernetes.io/instance"] = in.Spec.ClusterID
		labels["app.kubernetes.io/version"] = in.Spec.Config.Version
	}
	labels["app.kubernetes.io/component"] = component
	labels["app.kubernetes.io/part-of"] = "embedded-cluster"
	labels["app.kubernetes.io/managed-by"] = "embedded-cluster-operator"
	return labels
}
