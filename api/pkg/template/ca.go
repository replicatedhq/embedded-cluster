package template

// privateCACert returns the name of a ConfigMap containing additional CA certificates
// provided by the host system. In Embedded Cluster, this ConfigMap is automatically
// created and managed when a host CA bundle is detected. Returns empty string if
// no ConfigMap is available (e.g., in Kubernetes installations).
func (e *Engine) privateCACert() string {
	return e.privateCACertConfigMapName
}
