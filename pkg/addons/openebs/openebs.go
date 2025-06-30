package openebs

import (
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
)

const (
	_releaseName = "openebs"
	_namespace   = "openebs"
)

var _ types.AddOn = (*OpenEBS)(nil)

type OpenEBS struct {
	OpenEBSDataDir string
}

func (o *OpenEBS) Name() string {
	return "Storage"
}

func (o *OpenEBS) Version() string {
	return Metadata.Version
}

func (o *OpenEBS) ReleaseName() string {
	return _releaseName
}

func (o *OpenEBS) Namespace() string {
	return _namespace
}

func (o *OpenEBS) ChartLocation(domains ecv1beta1.Domains) string {
	if domains.ProxyRegistryDomain == "" {
		return Metadata.Location
	}
	return strings.Replace(Metadata.Location, "proxy.replicated.com", domains.ProxyRegistryDomain, 1)
}
