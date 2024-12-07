package velero

import (
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

func (v *Velero) Prepare() error {
	if err := v.generateHelmValues(); err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	return nil
}

func (v *Velero) generateHelmValues() error {
	if v.Proxy != nil {
		helmValues["configuration"] = map[string]interface{}{
			"extraEnvVars": map[string]string{
				"HTTP_PROXY":  v.Proxy.HTTPProxy,
				"HTTPS_PROXY": v.Proxy.HTTPSProxy,
				"NO_PROXY":    v.Proxy.NoProxy,
			},
		}
	}

	var err error
	podVolumePath := filepath.Join(runtimeconfig.EmbeddedClusterK0sSubDir(), "kubelet/pods")
	helmValues, err = helm.SetValue(helmValues, "nodeAgent.podVolumePath", podVolumePath)
	if err != nil {
		return errors.Wrap(err, "set helm value nodeAgent.podVolumePath")
	}

	return nil
}
