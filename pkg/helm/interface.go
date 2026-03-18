package helm

import (
	"context"

	"helm.sh/helm/v3/pkg/repo"
	chart "helm.sh/helm/v4/pkg/chart/v2"
)

var (
	_clientFactory ClientFactory
)

type Client interface {
	Close() error
	AddRepo(ctx context.Context, repo *repo.Entry) error
	AddRepoBin(ctx context.Context, repo *repo.Entry) error
	Latest(ctx context.Context, reponame, chart string) (string, error)
	Pull(ctx context.Context, reponame, chart string, version string) (string, error)
	PullByRef(ctx context.Context, ref string, version string) (string, error)
	RegistryAuth(ctx context.Context, server, user, pass string) error
	Push(ctx context.Context, path, dst string) error
	GetChartMetadata(ctx context.Context, ref string, version string) (*chart.Metadata, error)
	ReleaseExists(ctx context.Context, namespace string, releaseName string) (bool, error)
	Install(ctx context.Context, opts InstallOptions) (*ReleaseInfo, error)
	Upgrade(ctx context.Context, opts UpgradeOptions) (*ReleaseInfo, error)
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
