package kubeutils

import (
	autopilotv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

var Scheme = scheme.Scheme

var Codecs = scheme.Codecs

var ParameterCodec = scheme.ParameterCodec

func init() {
	utilruntime.Must(autopilotv1beta2.AddToScheme(Scheme))
	utilruntime.Must(k0sv1beta1.AddToScheme(Scheme))
	utilruntime.Must(embeddedclusterv1beta1.AddToScheme(Scheme))
	utilruntime.Must(velerov1.AddToScheme(Scheme))
}
