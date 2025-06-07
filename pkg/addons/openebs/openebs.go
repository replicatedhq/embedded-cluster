package openebs

import (
	_ "embed"
	"log/slog"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
)

const (
	releaseName = "openebs"
	namespace   = "openebs"
)

var _ types.AddOn = (*OpenEBS)(nil)

type OpenEBS struct {
	logf types.LogFunc

	dryRunManifests [][]byte
}

type Option func(*OpenEBS)

func New(opts ...Option) *OpenEBS {
	addon := &OpenEBS{}
	for _, opt := range opts {
		opt(addon)
	}
	if addon.logf == nil {
		addon.logf = slog.Info
	}
	return addon
}

func WithLogFunc(logf types.LogFunc) Option {
	return func(a *OpenEBS) {
		a.logf = logf
	}
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

func (o *OpenEBS) ChartLocation(domains ecv1beta1.Domains) string {
	if domains.ProxyRegistryDomain == "" {
		return Metadata.Location
	}
	return strings.Replace(Metadata.Location, "proxy.replicated.com", domains.ProxyRegistryDomain, 1)
}
