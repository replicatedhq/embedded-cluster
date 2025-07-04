package adminconsole

import (
	"context"
	_ "embed"
	"strings"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	//go:embed static/values.tpl.yaml
	rawvalues []byte
)

func (a *AdminConsole) GenerateHelmValues(ctx context.Context, kcli client.Client, domains ecv1beta1.Domains, overrides []string) (map[string]interface{}, error) {
	hv, err := helmValues()
	if err != nil {
		return nil, errors.Wrap(err, "get helm values")
	}

	marshalled, err := helm.MarshalValues(hv)
	if err != nil {
		return nil, errors.Wrap(err, "marshal helm values")
	}

	// replace proxy.replicated.com with the potentially customized proxy registry domain
	if domains.ProxyRegistryDomain != "" {
		marshalled = strings.ReplaceAll(marshalled, "proxy.replicated.com", domains.ProxyRegistryDomain)
	}

	copiedValues, err := helm.UnmarshalValues(marshalled)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshal helm values")
	}

	if a.isEmbeddedCluster() {
		// embeddedClusterID controls whether the admin console thinks it is running in an embedded cluster
		copiedValues["embeddedClusterID"] = a.ClusterID
		copiedValues["embeddedClusterDataDir"] = a.DataDir
		copiedValues["embeddedClusterK0sDir"] = a.K0sDataDir
	}

	copiedValues["isHA"] = a.IsHA
	copiedValues["isMultiNodeEnabled"] = a.IsMultiNodeEnabled
	copiedValues["isAirgap"] = a.IsAirgap

	if domains.ReplicatedAppDomain != "" {
		copiedValues["replicatedAppEndpoint"] = netutils.MaybeAddHTTPS(domains.ReplicatedAppDomain)
	}
	if domains.ReplicatedRegistryDomain != "" {
		copiedValues["replicatedRegistryDomain"] = domains.ReplicatedRegistryDomain
	}
	if domains.ProxyRegistryDomain != "" {
		copiedValues["proxyRegistryDomain"] = domains.ProxyRegistryDomain
	}

	extraEnv := []map[string]interface{}{}

	// currently, the admin console only supports improved disaster recovery in embedded clusters
	if a.isEmbeddedCluster() {
		extraEnv = append(extraEnv,
			map[string]interface{}{
				"name":  "ENABLE_IMPROVED_DR",
				"value": "true",
			},
			map[string]interface{}{
				"name":  "SSL_CERT_CONFIGMAP",
				"value": privateCASConfigMapName,
			},
		)
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

	err = helm.SetValue(copiedValues, "kurlProxy.nodePort", a.AdminConsolePort)
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

func helmValues() (map[string]interface{}, error) {
	hv, err := release.RenderHelmValues(rawvalues, Metadata)
	if err != nil {
		return nil, errors.Wrap(err, "render helm values")
	}

	hv["embeddedClusterVersion"] = versions.Version

	if AdminConsoleImageOverride != "" {
		hv["images"].(map[string]any)["kotsadm"] = AdminConsoleImageOverride
	}
	if AdminConsoleMigrationsImageOverride != "" {
		hv["images"].(map[string]any)["migrations"] = AdminConsoleMigrationsImageOverride
	}
	if AdminConsoleKurlProxyImageOverride != "" {
		hv["images"].(map[string]any)["kurlProxy"] = AdminConsoleKurlProxyImageOverride
	}

	return hv, nil
}
