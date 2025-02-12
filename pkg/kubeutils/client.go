package kubeutils

import (
	"fmt"

	autopilotv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	k0shelmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var Scheme = scheme.Scheme
var Codecs = scheme.Codecs
var ParameterCodec = scheme.ParameterCodec

func init() {
	utilruntime.Must(embeddedclusterv1beta1.AddToScheme(Scheme))
	utilruntime.Must(autopilotv1beta2.AddToScheme(Scheme))
	utilruntime.Must(k0sv1beta1.AddToScheme(Scheme))
	utilruntime.Must(k0shelmv1beta1.AddToScheme(Scheme))
	utilruntime.Must(velerov1.AddToScheme(Scheme))
}

// KubeClient returns a new kubernetes client.
func (k *KubeUtils) KubeClient() (client.Client, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to process kubernetes config: %w", err)
	}
	return client.New(cfg, client.Options{})
}

// RESTClientGetterFactory is a factory function that can be used to create namespaced
// genericclioptions.RESTClientGetters.
func (k *KubeUtils) RESTClientGetterFactory(namespace string) genericclioptions.RESTClientGetter {
	cfgFlags := genericclioptions.NewConfigFlags(false)
	if namespace != "" {
		cfgFlags.Namespace = &namespace
	}
	return cfgFlags
}

func GetClientset() (*kubernetes.Clientset, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("get kubernetes client config: %w", err)
	}

	return kubernetes.NewForConfig(cfg)
}
