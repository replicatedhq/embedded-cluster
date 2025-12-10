package api

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/internal/clients"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
)

func (a *API) initClients() error {
	if a.hcli == nil {
		if err := a.initHelmClient(); err != nil {
			return fmt.Errorf("init helm client: %w", err)
		}
	}
	if a.kcli == nil {
		if err := a.initKubeClient(); err != nil {
			return fmt.Errorf("init kube client: %w", err)
		}
	}
	return nil
}

// initKubeClient initializes the Kubernetes client
func (a *API) initKubeClient() error {
	// We can only initialize the kube client in upgrade mode as other modes may not have a cluster yet
	switch a.cfg.Mode {
	case types.ModeUpgrade:
		kubeClient, err := clients.NewKubeClient(clients.KubeClientOptions{
			RESTClientGetter: a.cfg.RuntimeConfig.GetKubernetesEnvSettings().RESTClientGetter(),
		})
		if err != nil {
			return fmt.Errorf("create kube client: %w", err)
		}
		a.kcli = kubeClient
		return nil
	}
	return nil
}

// initHelmClient initializes the Helm client based on the installation target
func (a *API) initHelmClient() error {
	switch a.cfg.InstallTarget {
	case types.InstallTargetLinux:
		return a.initLinuxHelmClient()
	case types.InstallTargetKubernetes:
		return a.initKubernetesHelmClient()
	default:
		return fmt.Errorf("unsupported install target: %s", a.cfg.InstallTarget)
	}
}

// initLinuxHelmClient initializes the Helm client for Linux installations
func (a *API) initLinuxHelmClient() error {
	airgapPath := ""
	if a.cfg.AirgapBundle != "" {
		airgapPath = a.cfg.RuntimeConfig.EmbeddedClusterChartsSubDir()
	}

	hcli, err := helm.NewClient(helm.HelmOptions{
		HelmPath:              a.cfg.RuntimeConfig.PathToEmbeddedClusterBinary("helm"),
		KubernetesEnvSettings: a.cfg.RuntimeConfig.GetKubernetesEnvSettings(),
		K8sVersion:            versions.K0sVersion,
		AirgapPath:            airgapPath,
	})
	if err != nil {
		return fmt.Errorf("create linux helm client: %w", err)
	}

	a.hcli = hcli
	return nil
}

// initKubernetesHelmClient initializes the Helm client for Kubernetes installations
func (a *API) initKubernetesHelmClient() error {
	// get the kubernetes version
	kcli, err := clients.NewDiscoveryClient(clients.KubeClientOptions{
		RESTClientGetter: a.cfg.Installation.GetKubernetesEnvSettings().RESTClientGetter(),
	})
	if err != nil {
		return fmt.Errorf("create discovery client: %w", err)
	}
	k8sVersion, err := kcli.ServerVersion()
	if err != nil {
		return fmt.Errorf("get server version: %w", err)
	}

	// get the helm binary path
	helmPath, err := a.cfg.Installation.PathToEmbeddedBinary("helm")
	if err != nil {
		return fmt.Errorf("get helm path: %w", err)
	}

	// create the helm client
	hcli, err := helm.NewClient(helm.HelmOptions{
		HelmPath:              helmPath,
		KubernetesEnvSettings: a.cfg.Installation.GetKubernetesEnvSettings(),
		// TODO: how can we support airgap?
		AirgapPath: "",
		K8sVersion: k8sVersion.String(),
	})
	if err != nil {
		return fmt.Errorf("create kubernetes helm client: %w", err)
	}

	a.hcli = hcli
	return nil
}
