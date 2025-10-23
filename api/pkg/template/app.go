package template

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

// namespace returns the namespace for the app
func (e *Engine) namespace() string {
	kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(context.Background(), nil)
	if err != nil {
		return "kotsadm"
	}

	return kotsadmNamespace
}
