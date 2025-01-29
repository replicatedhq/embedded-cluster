package adminconsole

import (
	_ "embed"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

func (a *AdminConsole) prepare(overrides []string) error {
	if err := a.generateHelmValues(overrides); err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	return nil
}

func (a *AdminConsole) generateHelmValues(overrides []string) error {
	helmValues["embeddedClusterID"] = metrics.ClusterID().String()
	helmValues["isHA"] = a.IsHA

	if a.IsAirgap {
		helmValues["isAirgap"] = "true"
	} else {
		helmValues["isAirgap"] = "false"
	}

	if a.Proxy != nil {
		helmValues["extraEnv"] = []map[string]interface{}{
			{
				"name":  "HTTP_PROXY",
				"value": a.Proxy.HTTPProxy,
			},
			{
				"name":  "HTTPS_PROXY",
				"value": a.Proxy.HTTPSProxy,
			},
			{
				"name":  "NO_PROXY",
				"value": a.Proxy.NoProxy,
			},
		}
	} else {
		delete(helmValues, "extraEnv")
	}

	var err error
	helmValues, err = helm.SetValue(helmValues, "kurlProxy.nodePort", runtimeconfig.AdminConsolePort())
	if err != nil {
		return errors.Wrap(err, "set kurlProxy.nodePort")
	}

	for _, override := range overrides {
		helmValues, err = helm.PatchValues(helmValues, override)
		if err != nil {
			return errors.Wrap(err, "patch helm values")
		}
	}

	return nil
}
