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
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"gopkg.in/yaml.v3"
)

var (
	//go:embed static/values.tpl.yaml
	rawvalues []byte
	// helmValues is the unmarshal version of rawvalues.
	helmValues map[string]interface{}
)

func init() {
	if err := yaml.Unmarshal(rawmetadata, &Metadata); err != nil {
		panic(errors.Wrap(err, "unmarshal metadata"))
	}

	hv, err := release.RenderHelmValues(rawvalues, Metadata)
	if err != nil {
		panic(errors.Wrap(err, "unmarshal values"))
	}
	helmValues = hv

	helmValues["embeddedClusterVersion"] = versions.Version
}

func (a *AdminConsole) GenerateHelmValues(ctx context.Context, inSpec ecv1beta1.InstallationSpec, overrides []string) (map[string]interface{}, error) {
	rc := runtimeconfig.New(inSpec.RuntimeConfig)
	domains := runtimeconfig.GetDomains(inSpec.Config)

	// create a copy of the helm values so we don't modify the original
	marshalled, err := helm.MarshalValues(helmValues)
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

	copiedValues["embeddedClusterID"] = inSpec.ClusterID
	copiedValues["embeddedClusterDataDir"] = rc.EmbeddedClusterHomeDirectory()
	copiedValues["embeddedClusterK0sDir"] = rc.EmbeddedClusterK0sSubDir()
	copiedValues["isHA"] = inSpec.HighAvailability
	copiedValues["isMultiNodeEnabled"] = inSpec.LicenseInfo != nil && inSpec.LicenseInfo.IsMultiNodeEnabled

	if inSpec.AirGap {
		copiedValues["isAirgap"] = "true"
	} else {
		copiedValues["isAirgap"] = "false"
	}

	if domains.ReplicatedAppDomain != "" {
		copiedValues["replicatedAppEndpoint"] = netutils.MaybeAddHTTPS(domains.ReplicatedAppDomain)
	}
	if domains.ReplicatedRegistryDomain != "" {
		copiedValues["replicatedRegistryDomain"] = domains.ReplicatedRegistryDomain
	}
	if domains.ProxyRegistryDomain != "" {
		copiedValues["proxyRegistryDomain"] = domains.ProxyRegistryDomain
	}

	extraEnv := []map[string]interface{}{
		{
			"name":  "ENABLE_IMPROVED_DR",
			"value": "true",
		},
		{
			"name":  "SSL_CERT_CONFIGMAP",
			"value": "kotsadm-private-cas",
		},
	}

	if inSpec.Proxy != nil {
		extraEnv = append(extraEnv,
			map[string]interface{}{
				"name":  "HTTP_PROXY",
				"value": inSpec.Proxy.HTTPProxy,
			},
			map[string]interface{}{
				"name":  "HTTPS_PROXY",
				"value": inSpec.Proxy.HTTPSProxy,
			},
			map[string]interface{}{
				"name":  "NO_PROXY",
				"value": inSpec.Proxy.NoProxy,
			},
		)
	}

	extraVolumes := []map[string]interface{}{}
	extraVolumeMounts := []map[string]interface{}{}

	if rc.HostCABundlePath() != "" {
		extraVolumes = append(extraVolumes, map[string]interface{}{
			"name": "host-ca-bundle",
			"hostPath": map[string]interface{}{
				"path": rc.HostCABundlePath(),
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
