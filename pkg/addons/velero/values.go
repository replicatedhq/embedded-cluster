package velero

import (
	"context"
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	//go:embed static/values.tpl.yaml
	rawvalues []byte
)

func (v *Velero) GenerateHelmValues(ctx context.Context, kcli client.Client, domains ecv1beta1.Domains, overrides []string) (map[string]interface{}, error) {
	hv, err := helmValues()
	if err != nil {
		return nil, errors.Wrap(err, "get helm values")
	}

	// Inject custom Velero plugins from ConfigSpec before any further processing
	if err := v.injectPluginInitContainers(hv, domains); err != nil {
		return nil, errors.Wrap(err, "inject plugin init containers")
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

	copiedValues["nodeAgent"] = map[string]any{
		"extraVolumes":      extraVolumes,
		"extraVolumeMounts": extraVolumeMounts,
	}

	podVolumePath := filepath.Join(v.K0sDataDir, "kubelet/pods")
	err = helm.SetValue(copiedValues, "nodeAgent.podVolumePath", podVolumePath)
	if err != nil {
		return nil, errors.Wrap(err, "set helm value nodeAgent.podVolumePath")
	}
	pluginVolumePath := filepath.Join(v.K0sDataDir, "kubelet/plugins")
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

func helmValues() (map[string]interface{}, error) {
	hv, err := release.RenderHelmValues(rawvalues, Metadata)
	if err != nil {
		return nil, errors.Wrap(err, "render helm values")
	}

	return hv, nil
}

// injectPluginInitContainers injects custom Velero plugin initContainers from ConfigSpec
func (v *Velero) injectPluginInitContainers(values map[string]interface{}, domains ecv1beta1.Domains) error {
	// Only use plugins from EmbeddedConfigSpec
	// EndUserConfigSpec is only used for overrides (via addOnOverrides mechanism)
	if v.EmbeddedConfigSpec == nil || len(v.EmbeddedConfigSpec.Extensions.Velero.Plugins) == 0 {
		return nil
	}

	allPlugins := v.EmbeddedConfigSpec.Extensions.Velero.Plugins

	// Get existing initContainers or create empty slice
	var existingInitContainers []any
	if existing, ok := values["initContainers"]; ok {
		if containers, ok := existing.([]any); ok {
			existingInitContainers = containers
		}
	}

	// Process each plugin and create initContainer
	for _, plugin := range allPlugins {
		processedImage := v.processPluginImage(plugin.Image, domains)
		imagePullPolicy := plugin.ImagePullPolicy
		if imagePullPolicy == "" {
			imagePullPolicy = "IfNotPresent" // Default to match AWS plugin
		}

		initContainer := v.generatePluginContainer(plugin.Name, processedImage, imagePullPolicy)
		existingInitContainers = append(existingInitContainers, initContainer)
	}

	// Update values with merged initContainers
	values["initContainers"] = existingInitContainers
	return nil
}

// processPluginImage processes plugin image according to registry prepending rules
// Priority:
// 1. If image contains "/", use as-is (explicit registry)
// 2. If domains.ProxyRegistryDomain is set, prepend it
// 3. Default to proxy.replicated.com
func (v *Velero) processPluginImage(image string, domains ecv1beta1.Domains) string {
	// If image already has a registry (contains "/"), use as-is
	if strings.Contains(image, "/") {
		return image
	}

	// Use custom proxy registry domain if configured
	if domains.ProxyRegistryDomain != "" {
		return fmt.Sprintf("%s/%s", domains.ProxyRegistryDomain, image)
	}

	// Default to Replicated proxy registry
	return fmt.Sprintf("proxy.replicated.com/%s", image)
}

// generatePluginContainer creates an initContainer spec for a Velero plugin
func (v *Velero) generatePluginContainer(name, image, imagePullPolicy string) map[string]interface{} {
	return map[string]interface{}{
		"name":            name,
		"image":           image,
		"imagePullPolicy": imagePullPolicy,
		"volumeMounts": []map[string]interface{}{
			{
				"mountPath": "/target",
				"name":      "plugins",
			},
		},
	}
}
