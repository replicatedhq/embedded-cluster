package embeddedclusteroperator

import (
	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

func (e *EmbeddedClusterOperator) prepare(overrides []string) error {
	if err := e.generateHelmValues(overrides); err != nil {
		return errors.Wrap(err, "generate helm values")
	}

	return nil
}

func (e *EmbeddedClusterOperator) generateHelmValues(overrides []string) error {
	if e.BinaryNameOverride != "" {
		helmValues["embeddedBinaryName"] = e.BinaryNameOverride
	} else {
		helmValues["embeddedBinaryName"] = runtimeconfig.BinaryName()
	}

	helmValues["embeddedClusterID"] = metrics.ClusterID().String()

	if e.IsAirgap {
		helmValues["isAirgap"] = "true"
	}

	if e.Proxy != nil {
		helmValues["extraEnv"] = []map[string]interface{}{
			{
				"name":  "HTTP_PROXY",
				"value": e.Proxy.HTTPProxy,
			},
			{
				"name":  "HTTPS_PROXY",
				"value": e.Proxy.HTTPSProxy,
			},
			{
				"name":  "NO_PROXY",
				"value": e.Proxy.NoProxy,
			},
		}
	} else {
		delete(helmValues, "extraEnv")
	}

	//for _, override := range overrides {
	//	var err error
	//	helmValues, err = helm.PatchValues(helmValues, override)
	//	if err != nil {
	//		return errors.Wrap(err, "patch helm values")
	//	}
	//}

	return nil
}
