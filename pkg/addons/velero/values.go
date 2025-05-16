package velero

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
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

	extraEnvVars := map[string]any{}
	extraVolumes := []string{}
	extraVolumeMounts := []string{}

	if v.Proxy != nil {
		extraEnvVars["HTTP_PROXY"] = v.Proxy.HTTPProxy
		extraEnvVars["HTTPS_PROXY"] = v.Proxy.HTTPSProxy
		extraEnvVars["NO_PROXY"] = v.Proxy.NoProxy
	}

	if v.HostCABundlePath != "" {
		extraVolume, err := yaml.Marshal(corev1.Volume{
			Name: "host-ca-bundle",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: v.HostCABundlePath,
					Type: ptr.To(corev1.HostPathFileOrCreate),
				},
			},
		})
		if err != nil {
			return nil, errors.Wrap(err, "marshal extra volume")
		}
		extraVolumes = append(extraVolumes, string(extraVolume))

		extraVolumeMount, err := yaml.Marshal(corev1.VolumeMount{
			Name:      "host-ca-bundle",
			MountPath: "/certs/ca-certificates.crt",
		})
		if err != nil {
			return nil, errors.Wrap(err, "marshal extra volume mounts")
		}
		extraVolumeMounts = append(extraVolumeMounts, string(extraVolumeMount))

		extraEnvVars["SSL_CERT_DIR"] = "/certs"
	}

	copiedValues["configuration"] = map[string]any{
		"extraEnvVars": extraEnvVars,
	}
	copiedValues["extraVolumes"] = extraVolumes
	copiedValues["extraVolumeMounts"] = extraVolumeMounts

	podVolumePath := filepath.Join(runtimeconfig.EmbeddedClusterK0sSubDir(), "kubelet/pods")
	err = helm.SetValue(copiedValues, "nodeAgent.podVolumePath", podVolumePath)
	if err != nil {
		return nil, errors.Wrap(err, "set helm value nodeAgent.podVolumePath")
	}

	for _, override := range overrides {
		copiedValues, err = helm.PatchValues(copiedValues, override)
		if err != nil {
			return nil, errors.Wrap(err, "patch helm values")
		}
	}

	return copiedValues, nil
}
