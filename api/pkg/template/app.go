package template

import (
	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
)

// namespace returns the namespace for the app
func (e *Engine) namespace() string {
	return constants.KotsadmNamespace
}
