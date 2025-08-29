package kubernetes

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/internal/clients"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// getK8sVersion creates a kubernetes client and returns the kubernetes version
func getK8sVersion(restClientGetter genericclioptions.RESTClientGetter) (string, error) {
	kcli, err := clients.NewDiscoveryClient(clients.KubeClientOptions{
		RESTClientGetter: restClientGetter,
	})
	if err != nil {
		return "", fmt.Errorf("create discovery client: %w", err)
	}
	version, err := kcli.ServerVersion()
	if err != nil {
		return "", fmt.Errorf("get server version: %w", err)
	}
	return version.String(), nil
}
