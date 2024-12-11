package adminconsole

import (
	_ "embed"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
)

func (a *AdminConsole) prepare() error {
	if err := a.generateHelmValues(); err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	return nil
}

func (a *AdminConsole) generateHelmValues() error {
	helmValues["embeddedClusterVersion"] = versions.Version
	helmValues["embeddedClusterID"] = metrics.ClusterID().String()
	helmValues["isHA"] = a.IsHA

	if a.IsAirgap || a.AirgapBundle != "" {
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

	return nil
}
