package embeddedclusteroperator

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (e *EmbeddedClusterOperator) GenerateHelmValues(ctx context.Context, kcli client.Client, overrides []string) (map[string]interface{}, error) {
	// create a copy of the helm values so we don't modify the original
	marshalled, err := helm.MarshalValues(helmValues)
	if err != nil {
		return nil, errors.Wrap(err, "marshal helm values")
	}

	// replace proxy.replicated.com with the potentially customized proxy registry domain
	if e.ProxyRegistryDomain != "" {
		marshalled = strings.ReplaceAll(marshalled, "proxy.replicated.com", e.ProxyRegistryDomain)
	}

	copiedValues, err := helm.UnmarshalValues(marshalled)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshal helm values")
	}

	if e.ImageRepoOverride != "" {
		copiedValues["image"].(map[string]interface{})["repository"] = e.ImageRepoOverride
	}
	if e.ImageTagOverride != "" {
		copiedValues["image"].(map[string]interface{})["tag"] = e.ImageTagOverride
	}
	if e.UtilsImageOverride != "" {
		copiedValues["utilsImage"] = e.UtilsImageOverride
	}

	copiedValues["embeddedClusterID"] = metrics.ClusterID().String()

	if e.IsAirgap {
		copiedValues["isAirgap"] = "true"
	}

	extraEnvVars := []map[string]any{}
	extraVolumes := []map[string]any{}
	extraVolumeMounts := []map[string]any{}

	if e.Proxy != nil {
		extraEnvVars = append(extraEnvVars, []map[string]any{
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
		}...)
	}

	if e.HostCABundlePath != "" {
		extraVolumes = append(extraVolumes, map[string]any{
			"name": "host-ca-bundle",
			"hostPath": map[string]any{
				"path": e.HostCABundlePath,
				"type": "FileOrCreate",
			},
		})

		extraVolumeMounts = append(extraVolumeMounts, map[string]any{
			"name":      "host-ca-bundle",
			"mountPath": "/certs/ca-certificates.crt",
		})

		extraEnvVars = append(extraEnvVars, map[string]any{
			"name":  "SSL_CERT_DIR",
			"value": "/certs",
		})
	}

	copiedValues["extraEnv"] = extraEnvVars
	copiedValues["extraVolumes"] = extraVolumes
	copiedValues["extraVolumeMounts"] = extraVolumeMounts

	for _, override := range overrides {
		var err error
		copiedValues, err = helm.PatchValues(copiedValues, override)
		if err != nil {
			return nil, errors.Wrap(err, "patch helm values")
		}
	}

	return copiedValues, nil
}
