package registry

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ types.AddOn = (*Registry)(nil)

type Registry struct {
	ServiceCIDR string
	IsHA        bool
}

const (
	_releaseName = "docker-registry"
	_namespace   = constants.RegistryNamespace

	_tlsSecretName    = "registry-tls"
	_lowerBandIPIndex = 10
)

var (
	// registryPassword is the password for the registry.
	registryPassword = helpers.RandString(20)
)

func (r *Registry) Name() string {
	return "Registry"
}

func (r *Registry) Version() string {
	return Metadata.Version
}

func (r *Registry) ReleaseName() string {
	return _releaseName
}

func (r *Registry) Namespace() string {
	return _namespace
}

func GetRegistryPassword() string {
	return registryPassword
}

// GetRegistryClusterIP returns the cluster IP for the registry service.
// This function is deterministic.
func GetRegistryClusterIP(serviceCIDR string) (string, error) {
	svcIP, err := helpers.GetLowerBandIP(serviceCIDR, _lowerBandIPIndex)
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

func (r *Registry) ChartLocation(domains ecv1beta1.Domains) string {
	if domains.ProxyRegistryDomain == "" {
		return Metadata.Location
	}
	return strings.Replace(Metadata.Location, "proxy.replicated.com", domains.ProxyRegistryDomain, 1)
}

// IsRegistryHA checks if the registry deployment has greater than 1 replica.
func IsRegistryHA(ctx context.Context, kcli client.Client) (bool, error) {
	deploy := appsv1.Deployment{}
	err := kcli.Get(ctx, client.ObjectKey{Namespace: _namespace, Name: "registry"}, &deploy)
	if err != nil {
		return false, fmt.Errorf("get registry deployment: %w", err)
	}

	return deploy.Spec.Replicas != nil && *deploy.Spec.Replicas > 1, nil
}
