package kubeutils

import (
	autopilotv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

func init() {
	utilruntime.Must(autopilotv1beta2.AddToScheme(scheme.Scheme))
	utilruntime.Must(k0sv1beta1.AddToScheme(scheme.Scheme))
	utilruntime.Must(embeddedclusterv1beta1.AddToScheme(scheme.Scheme))
}
