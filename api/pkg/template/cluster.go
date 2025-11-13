package template

import (
	"context"

	corev1 "k8s.io/api/core/v1"
)

// nodeCount returns the total count of Kubernetes nodes in the cluster.
// It returns 0 if the kubeClient is unavailable or if an error occurs.
func (e *Engine) nodeCount() int {
	logger := e.logger.WithField("function", "node-count")
	if e.kubeClient == nil {
		logger.Warn("kube client is nil, cannot get node count")
		return 0
	}
	ctx := context.TODO()
	nodeList := &corev1.NodeList{}
	if err := e.kubeClient.List(ctx, nodeList); err != nil {
		logger.WithError(err).Warn("failed to list nodes")
		return 0
	}
	return len(nodeList.Items)
}
