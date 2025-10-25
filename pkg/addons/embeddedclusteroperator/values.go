package embeddedclusteroperator

import (
	"context"
	_ "embed"
	"strings"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	//go:embed static/values.tpl.yaml
	rawvalues []byte
)

func (e *EmbeddedClusterOperator) GenerateHelmValues(ctx context.Context, kcli client.Client, domains ecv1beta1.Domains, overrides []string) (map[string]interface{}, error) {
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

	if e.ImageRepoOverride != "" {
		copiedValues["image"].(map[string]interface{})["repository"] = e.ImageRepoOverride
	}
	if e.ImageTagOverride != "" {
		copiedValues["image"].(map[string]interface{})["tag"] = e.ImageTagOverride
	}
	if e.UtilsImageOverride != "" {
		copiedValues["utilsImage"] = e.UtilsImageOverride
	}

	copiedValues["embeddedClusterID"] = e.ClusterID

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

		extraEnvVars = append(extraEnvVars, []map[string]any{
			{
				"name":  "SSL_CERT_DIR",
				"value": "/certs",
			},
			{
				"name":  "PRIVATE_CA_BUNDLE_PATH",
				"value": "/certs/ca-certificates.crt",
			},
		}...)
	}

	if e.KotsadmNamespace != "" {
		extraEnvVars = append(extraEnvVars, map[string]any{
			"name":  "KOTSADM_NAMESPACE",
			"value": e.KotsadmNamespace,
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

func helmValues() (map[string]interface{}, error) {
	hv, err := release.RenderHelmValues(rawvalues, Metadata)
	if err != nil {
		return nil, errors.Wrap(err, "render helm values")
	}

	hv["embeddedClusterVersion"] = versions.Version
	hv["embeddedClusterK0sVersion"] = versions.K0sVersion

	return hv, nil
}
