package k8sutil

import (
	"fmt"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	autopilotv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	k0shelm "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	newScheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(scheme.AddToScheme(newScheme))

	utilruntime.Must(embeddedclusterv1beta1.AddToScheme(newScheme))
	utilruntime.Must(autopilotv1beta2.AddToScheme(newScheme))
	utilruntime.Must(k0sv1beta1.AddToScheme(newScheme))
	utilruntime.Must(k0shelm.AddToScheme(newScheme))
}

func Scheme() *runtime.Scheme {
	return newScheme
}

// KubeClient returns a new kubernetes client.
func KubeClient() (client.Client, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to process kubernetes config: %w", err)
	}
	return client.New(cfg, client.Options{Scheme: newScheme})
}
