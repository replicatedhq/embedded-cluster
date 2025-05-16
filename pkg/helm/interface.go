package helm

import (
	"context"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
)

var (
	_clientFactory ClientFactory
)

type Client interface {
	Close() error
	AddRepo(repo *repo.Entry) error
	Latest(reponame, chart string) (string, error)
	Pull(reponame, chart string, version string) (string, error)
	PullByRef(ref string, version string) (string, error)
	RegistryAuth(server, user, pass string) error
	Push(path, dst string) error
	GetChartMetadata(chartPath string) (*chart.Metadata, error)
	ReleaseExists(ctx context.Context, namespace string, releaseName string) (bool, error)
	Install(ctx context.Context, opts InstallOptions) (*release.Release, error)
	Upgrade(ctx context.Context, opts InstallOptions) (*release.Release, error)
	Uninstall(ctx context.Context, opts UninstallOptions) error
	Render(ctx context.Context, opts InstallOptions) ([][]byte, error)
}

type ClientFactory func(opts HelmOptions) (Client, error)

func SetClientFactory(fn ClientFactory) {
	_clientFactory = fn
}

func NewClient(opts HelmOptions) (Client, error) {
	if _clientFactory != nil {
		return _clientFactory(opts)
	}
	return newClient(opts)
}
