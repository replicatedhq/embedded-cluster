package cmd

import (
	"context"
	"fmt"
	"os"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	cmdutil "github.com/replicatedhq/embedded-cluster/pkg/cmd/util"
	"github.com/replicatedhq/embedded-cluster/pkg/configutils"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
)

// discoverKubeconfigPath finds the kubeconfig path from the k0s status command if it's available.
// Otherwise, it will use the data dir from the cli flag or the default path.
func discoverKubeconfigPath(ctx context.Context, runtimeConfig *ecv1beta1.RuntimeConfigSpec) (string, error) {
	var kubeconfigPath string

	status, err := getK0sStatus(ctx)
	if err == nil {
		kubeconfigPath = status.Vars.AdminKubeConfigPath
	} else {
		// Use the data dir from the cli flag if it's set
		if runtimeConfig.DataDir != "" {
			provider := defaults.NewProviderFromRuntimeConfig(runtimeConfig)
			kubeconfigPath = provider.PathToKubeConfig()
		} else {
			provider := defaults.NewProvider(ecv1beta1.DefaultDataDir)
			kubeconfigPath = provider.PathToKubeConfig()
		}
	}
	if _, err := os.Stat(kubeconfigPath); err != nil {
		return "", fmt.Errorf("kubeconfig not found at %s", kubeconfigPath)
	}

	return kubeconfigPath, nil
}

// discoverBestProvider discovers the provider from the cluster (if it's up) and will fall back to
// the /etc/embedded-cluster/ec.yaml file, the filesystem, or the default.
func discoverBestProvider(ctx context.Context) *defaults.Provider {
	// It's possible that the cluster is not up
	provider, err := getProviderFromCluster(ctx)
	if err == nil {
		return provider
	}

	// There might be a runtime config file
	runtimeConfig, err := configutils.ReadRuntimeConfig()
	if err == nil {
		provider = defaults.NewProviderFromRuntimeConfig(runtimeConfig)
		return provider
	}

	// Otherwise, fall back to the filesystem
	provider, err = cmdutil.NewProviderFromFilesystem()
	if err == nil {
		return provider
	}

	// If we can't find a provider, use the default
	return defaults.NewProvider(ecv1beta1.DefaultDataDir)
}

// getProviderFromCluster finds the kubeconfig and discovers the provider from the cluster. If this
// is a prior version of EC, we will have to fall back to the filesystem.
func getProviderFromCluster(ctx context.Context) (*defaults.Provider, error) {
	status, err := getK0sStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get k0s status: %w", err)
	}
	kubeconfigPath := status.Vars.AdminKubeConfigPath

	os.Setenv("KUBECONFIG", kubeconfigPath)

	// Discover the provider from the cluster
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return nil, fmt.Errorf("unable to create kube client: %w", err)
	}

	provider, err := cmdutil.NewProviderFromCluster(ctx, kcli)
	if err != nil {
		return nil, fmt.Errorf("unable to get config from cluster: %w", err)
	}
	return provider, nil
}
