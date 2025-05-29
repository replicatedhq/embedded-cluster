package velero

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (v *Velero) GenerateHelmValues(ctx context.Context, kcli client.Client, overrides []string) (map[string]interface{}, error) {
	// create a copy of the helm values so we don't modify the original
	marshalled, err := helm.MarshalValues(helmValues)
	if err != nil {
		return nil, errors.Wrap(err, "marshal helm values")
	}

	// replace proxy.replicated.com with the potentially customized proxy registry domain
	if v.ProxyRegistryDomain != "" {
		marshalled = strings.ReplaceAll(marshalled, "proxy.replicated.com", v.ProxyRegistryDomain)
	}

	copiedValues, err := helm.UnmarshalValues(marshalled)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshal helm values")
	}

	extraEnvVars := []map[string]any{}
	extraVolumes := []map[string]any{}
	extraVolumeMounts := []map[string]any{}

	if v.Proxy != nil {
		extraEnvVars = append(extraEnvVars, []map[string]any{
			{
				"name":  "HTTP_PROXY",
				"value": v.Proxy.HTTPProxy,
			},
			{
				"name":  "HTTPS_PROXY",
				"value": v.Proxy.HTTPSProxy,
			},
			{
				"name":  "NO_PROXY",
				"value": v.Proxy.NoProxy,
			},
		}...)
	}

	if v.HostCABundlePath != "" {
		extraVolumes = append(extraVolumes, map[string]any{
			"name": "host-ca-bundle",
			"hostPath": map[string]any{
				"path": v.HostCABundlePath,
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

	copiedValues["configuration"] = map[string]any{
		"extraEnvVars": extraEnvVars,
	}
	copiedValues["extraVolumes"] = extraVolumes
	copiedValues["extraVolumeMounts"] = extraVolumeMounts

	podVolumePath := filepath.Join(v.EmbeddedClusterK0sSubDir, "kubelet/pods")
	err = helm.SetValue(copiedValues, "nodeAgent.podVolumePath", podVolumePath)
	if err != nil {
		return nil, errors.Wrap(err, "set helm value nodeAgent.podVolumePath")
	}
	pluginVolumePath := filepath.Join(v.EmbeddedClusterK0sSubDir, "kubelet/plugins")
	err = helm.SetValue(copiedValues, "nodeAgent.pluginVolumePath", pluginVolumePath)
	if err != nil {
		return nil, errors.Wrap(err, "set helm value nodeAgent.pluginVolumePath")
	}

	for _, override := range overrides {
		copiedValues, err = helm.PatchValues(copiedValues, override)
		if err != nil {
			return nil, errors.Wrap(err, "patch helm values")
		}
	}

	return copiedValues, nil
}
