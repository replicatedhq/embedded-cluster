package template

import "github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"

// namespace returns the namespace for the app
func (e *Engine) namespace() string {
	kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(nil, nil)
	if err != nil {
		return "kotsadm"
	}

	return kotsadmNamespace
}
