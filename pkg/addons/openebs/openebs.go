package openebs

import (
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
)

const (
	releaseName = "openebs"
	namespace   = "openebs"
)

var _ types.AddOn = (*OpenEBS)(nil)

type OpenEBS struct {
	ProxyRegistryDomain string
}

func (o *OpenEBS) Name() string {
	return "Storage"
}

func (o *OpenEBS) Version() string {
	return Metadata.Version
}

func (o *OpenEBS) ReleaseName() string {
	return releaseName
}

func (o *OpenEBS) Namespace() string {
	return namespace
}

func (o *OpenEBS) ChartLocation() string {
	if o.ProxyRegistryDomain == "" {
		return Metadata.Location
	}
	return strings.Replace(Metadata.Location, "proxy.replicated.com", o.ProxyRegistryDomain, 1)
}
