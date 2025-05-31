package adminconsole

import (
	"context"
	_ "embed"
	"strings"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/netutil"
)

func (a *AdminConsole) GenerateHelmValues(ctx context.Context, opts types.InstallOptions, overrides []string) (map[string]interface{}, error) {
	// create a copy of the helm values so we don't modify the original
	marshalled, err := helm.MarshalValues(helmValues)
	if err != nil {
		return nil, errors.Wrap(err, "marshal helm values")
	}

	// replace proxy.replicated.com with the potentially customized proxy registry domain
	if opts.Domains.ProxyRegistryDomain != "" {
		marshalled = strings.ReplaceAll(marshalled, "proxy.replicated.com", opts.Domains.ProxyRegistryDomain)
	}

	copiedValues, err := helm.UnmarshalValues(marshalled)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshal helm values")
	}

	copiedValues["embeddedClusterID"] = opts.ClusterID
	copiedValues["embeddedClusterDataDir"] = opts.EmbeddedClusterHomeDir
	copiedValues["embeddedClusterK0sDir"] = opts.EmbeddedClusterK0sSubDir
	copiedValues["isHA"] = opts.IsHA
	copiedValues["isMultiNodeEnabled"] = opts.IsMultiNodeEnabled

	if opts.IsAirgap {
		copiedValues["isAirgap"] = "true"
	} else {
		copiedValues["isAirgap"] = "false"
	}

	if opts.Domains.ReplicatedAppDomain != "" {
		copiedValues["replicatedAppEndpoint"] = netutil.MaybeAddHTTPS(opts.Domains.ReplicatedAppDomain)
	}
	if opts.Domains.ReplicatedRegistryDomain != "" {
		copiedValues["replicatedRegistryDomain"] = opts.Domains.ReplicatedRegistryDomain
	}
	if opts.Domains.ProxyRegistryDomain != "" {
		copiedValues["proxyRegistryDomain"] = opts.Domains.ProxyRegistryDomain
	}

	extraEnv := []map[string]interface{}{
		{
			"name":  "ENABLE_IMPROVED_DR",
			"value": "true",
		},
	}

	if opts.Proxy != nil {
		extraEnv = append(extraEnv,
			map[string]interface{}{
				"name":  "HTTP_PROXY",
				"value": opts.Proxy.HTTPProxy,
			},
			map[string]interface{}{
				"name":  "HTTPS_PROXY",
				"value": opts.Proxy.HTTPSProxy,
			},
			map[string]interface{}{
				"name":  "NO_PROXY",
				"value": opts.Proxy.NoProxy,
			},
		)
	}

	extraVolumes := []map[string]interface{}{}
	extraVolumeMounts := []map[string]interface{}{}

	if opts.HostCABundlePath != "" {
		extraVolumes = append(extraVolumes, map[string]interface{}{
			"name": "host-ca-bundle",
			"hostPath": map[string]interface{}{
				"path": opts.HostCABundlePath,
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

	err = helm.SetValue(copiedValues, "kurlProxy.nodePort", opts.AdminConsolePort)
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
