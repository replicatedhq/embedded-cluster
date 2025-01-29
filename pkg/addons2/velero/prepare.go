package velero

import (
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

func (v *Velero) prepare(overrides []string) error {
	if err := v.generateHelmValues(overrides); err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	return nil
}

func (v *Velero) generateHelmValues(overrides []string) error {
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

	for _, override := range overrides {
		helmValues, err = helm.PatchValues(helmValues, override)
		if err != nil {
			return errors.Wrap(err, "patch helm values")
		}
	}

	return nil
}
