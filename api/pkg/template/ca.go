package template

import "github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"

// privateCACert returns the name of a ConfigMap containing additional CA certificates
// provided by the host system. In Embedded Cluster, this ConfigMap is automatically
// created and managed when a host CA bundle is detected.
// 
// This function returns the constant ConfigMap name since Embedded Cluster
// automatically manages the ConfigMap lifecycle. Future enhancements could check
// for actual ConfigMap existence or host CA bundle availability.
func (e *Engine) privateCACert() string {
	return adminconsole.PrivateCASConfigMapName
}