package registry

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Registry struct {
	ServiceCIDR         string
	IsHA                bool
	ProxyRegistryDomain string
}

const (
	releaseName      = "docker-registry"
	namespace        = runtimeconfig.RegistryNamespace
	tlsSecretName    = "registry-tls"
	lowerBandIPIndex = 10
)

var (
	//go:embed static/values.tpl.yaml
	rawvalues []byte
	// helmValues is the unmarshal version of rawvalues.
	helmValues map[string]interface{}
	//go:embed static/values-ha.tpl.yaml
	rawvaluesha []byte
	// helmValuesHA is the unmarshal version of rawvaluesha.
	helmValuesHA map[string]interface{}
	//go:embed static/metadata.yaml
	rawmetadata []byte
	// Metadata is the unmarshal version of rawmetadata.
	Metadata release.AddonMetadata
	// registryPassword is the password for the registry.
	registryPassword = helpers.RandString(20)
)

func init() {
	if err := yaml.Unmarshal(rawmetadata, &Metadata); err != nil {
		panic(errors.Wrap(err, "unable to unmarshal metadata"))
	}

	hv, err := release.RenderHelmValues(rawvalues, Metadata)
	if err != nil {
		panic(errors.Wrap(err, "unable to unmarshal values"))
	}
	helmValues = hv

	hvHA, err := release.RenderHelmValues(rawvaluesha, Metadata)
	if err != nil {
		panic(errors.Wrap(err, "unable to unmarshal ha values"))
	}
	helmValuesHA = hvHA
}

func (r *Registry) Name() string {
	return "Registry"
}

func (r *Registry) Version() string {
	return Metadata.Version
}

func (r *Registry) ReleaseName() string {
	return releaseName
}

func (r *Registry) Namespace() string {
	return namespace
}

func GetRegistryPassword() string {
	return registryPassword
}

// GetRegistryClusterIP returns the cluster IP for the registry service.
// This function is deterministic.
func GetRegistryClusterIP(serviceCIDR string) (string, error) {
	svcIP, err := helpers.GetLowerBandIP(serviceCIDR, lowerBandIPIndex)
	if err != nil {
		return "", errors.Wrap(err, "get cluster IP for registry service")
	}
	return svcIP.String(), nil
}

func getBackupLabels() map[string]string {
	return map[string]string{
		"app": "docker-registry",
	}
}

func (r *Registry) ChartLocation() string {
	if r.ProxyRegistryDomain == "" {
		return Metadata.Location
	}
	return strings.Replace(Metadata.Location, "proxy.replicated.com", r.ProxyRegistryDomain, 1)
}

// IsRegistryHA checks if the registry has been configured for HA by looking for the
// REGISTRY_STORAGE_S3_ACCESSKEY environment variable in the docker-registry container.
func IsRegistryHA(ctx context.Context, kcli client.Client) (bool, error) {
	deploy := appsv1.Deployment{}
	err := kcli.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "registry"}, &deploy)
	if err != nil {
		return false, fmt.Errorf("get registry deployment: %w", err)
	}

	for _, c := range deploy.Spec.Template.Spec.Containers {
		if c.Name == "docker-registry" {
			for _, env := range c.Env {
				if env.Name == "REGISTRY_STORAGE_S3_ACCESSKEY" &&
					env.ValueFrom.SecretKeyRef != nil &&
					env.ValueFrom.SecretKeyRef.Name == "seaweedfs-s3-rw" {
					return true, nil
				}
			}
		}
	}

	return false, nil
}
