package adminconsole

import (
	"context"
	_ "embed"
	"strings"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (a *AdminConsole) GenerateHelmValues(ctx context.Context, kcli client.Client, rc runtimeconfig.RuntimeConfig, overrides []string) (map[string]interface{}, error) {
	// create a copy of the helm values so we don't modify the original
	marshalled, err := helm.MarshalValues(helmValues)
	if err != nil {
		return nil, errors.Wrap(err, "marshal helm values")
	}

	// replace proxy.replicated.com with the potentially customized proxy registry domain
	if a.ProxyRegistryDomain != "" {
		marshalled = strings.ReplaceAll(marshalled, "proxy.replicated.com", a.ProxyRegistryDomain)
	}

	copiedValues, err := helm.UnmarshalValues(marshalled)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshal helm values")
	}

	copiedValues["embeddedClusterID"] = metrics.ClusterID().String()
	copiedValues["embeddedClusterDataDir"] = rc.EmbeddedClusterHomeDirectory()
	copiedValues["embeddedClusterK0sDir"] = rc.EmbeddedClusterK0sSubDir()
	copiedValues["isHA"] = a.IsHA
	copiedValues["isMultiNodeEnabled"] = a.IsMultiNodeEnabled

	if a.IsAirgap {
		copiedValues["isAirgap"] = "true"
	} else {
		copiedValues["isAirgap"] = "false"
	}

	if a.ReplicatedAppDomain != "" {
		copiedValues["replicatedAppEndpoint"] = netutils.MaybeAddHTTPS(a.ReplicatedAppDomain)
	}
	if a.ReplicatedRegistryDomain != "" {
		copiedValues["replicatedRegistryDomain"] = a.ReplicatedRegistryDomain
	}
	if a.ProxyRegistryDomain != "" {
		copiedValues["proxyRegistryDomain"] = a.ProxyRegistryDomain
	}

	extraEnv := []map[string]interface{}{
		{
			"name":  "ENABLE_IMPROVED_DR",
			"value": "true",
		},
	}

	if a.Proxy != nil {
		extraEnv = append(extraEnv,
			map[string]interface{}{
				"name":  "HTTP_PROXY",
				"value": a.Proxy.HTTPProxy,
			},
			map[string]interface{}{
				"name":  "HTTPS_PROXY",
				"value": a.Proxy.HTTPSProxy,
			},
			map[string]interface{}{
				"name":  "NO_PROXY",
				"value": a.Proxy.NoProxy,
			},
		)
	}

	extraVolumes := []map[string]interface{}{}
	extraVolumeMounts := []map[string]interface{}{}

	if a.HostCABundlePath != "" {
		extraVolumes = append(extraVolumes, map[string]interface{}{
			"name": "host-ca-bundle",
			"hostPath": map[string]interface{}{
				"path": a.HostCABundlePath,
				"type": "FileOrCreate",
			},
		})

		extraVolumeMounts = append(extraVolumeMounts, map[string]interface{}{
			"name":      "host-ca-bundle",
			"mountPath": "/certs/ca-certificates.crt",
		})

		extraEnv = append(extraEnv, map[string]interface{}{
			"name":  "SSL_CERT_DIR",
			"value": "/certs",
		})
	}

	copiedValues["extraEnv"] = extraEnv
	copiedValues["extraVolumes"] = extraVolumes
	copiedValues["extraVolumeMounts"] = extraVolumeMounts

	err = helm.SetValue(copiedValues, "kurlProxy.nodePort", rc.AdminConsolePort())
	if err != nil {
		return nil, errors.Wrap(err, "set kurlProxy.nodePort")
	}

	for _, override := range overrides {
		copiedValues, err = helm.PatchValues(copiedValues, override)
		if err != nil {
			return nil, errors.Wrap(err, "patch helm values")
		}
	}

	return copiedValues, nil
}
