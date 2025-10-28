package clients

import (
	"fmt"

	autopilotv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	k0shelmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	coreclientset "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var localSchemeBuilder = runtime.NewSchemeBuilder(
	apiextensionsv1.AddToScheme,
	embeddedclusterv1beta1.AddToScheme,
	// we are using the deprecated scheme for backwards compatibility, we can remove this once we stop supporting k0s v1.30
	autopilotv1beta2.Install,
	k0sv1beta1.Install,
	// we are using the deprecated scheme for backwards compatibility, we can remove this once we stop supporting k0s v1.30
	k0shelmv1beta1.Install,
	velerov1.AddToScheme,
)

type KubeClientOptions struct {
	RESTClientGetter genericclioptions.RESTClientGetter
	KubeConfigPath   string
}

func getScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	coreclientset.AddToScheme(s)
	localSchemeBuilder.AddToScheme(s)
	return s
}

// NewKubeClient returns a new kubernetes client.
func NewKubeClient(opts KubeClientOptions) (client.Client, error) {
	restConfig, err := getRESTConfig(opts)
	if err != nil {
		return nil, err
	}
	return client.New(restConfig, client.Options{Scheme: getScheme()})
}

// NewMetadataClient returns a new kube metadata client.
func NewMetadataClient(opts KubeClientOptions) (metadata.Interface, error) {
	restConfig, err := getRESTConfig(opts)
	if err != nil {
		return nil, err
	}
	return metadata.NewForConfig(restConfig)
}

// NewDiscoveryClient returns a new kube discovery client.
func NewDiscoveryClient(opts KubeClientOptions) (discovery.DiscoveryInterface, error) {
	restConfig, err := getRESTConfig(opts)
	if err != nil {
		return nil, err
	}
	return discovery.NewDiscoveryClientForConfig(restConfig)
}

func getRESTConfig(opts KubeClientOptions) (*rest.Config, error) {
	if opts.RESTClientGetter != nil {
		conf, err := opts.RESTClientGetter.ToRESTConfig()
		if err != nil {
			return nil, fmt.Errorf("invalid rest client getter: %w", err)
		}
		return conf, nil
	}

	if opts.KubeConfigPath != "" {
		conf, err := clientcmd.BuildConfigFromFlags("", opts.KubeConfigPath)
		if err != nil {
			return nil, fmt.Errorf("invalid kubeconfig path: %w", err)
		}
		return conf, nil
	}

	return nil, fmt.Errorf("a valid kube config is required to create a kube client")
}
