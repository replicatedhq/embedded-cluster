package template

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (e *Engine) nodeCount() (int, error) {
	if e.kcli == nil {
		return 0, fmt.Errorf("kubernetes client is nil")
	}

	ctx := context.Background()
	var nodeList corev1.NodeList
	if err := e.kcli.List(ctx, &nodeList, &client.ListOptions{}); err != nil {
		return 0, fmt.Errorf("failed to list nodes for NodeCount template function: %w", err)
	}

	return len(nodeList.Items), nil
}
