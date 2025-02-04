package helm

import (
	"context"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
)

var (
	_hc Client
)

func SetClient(hc Client) {
	_hc = hc
}

type Client interface {
	Close() error
	AddRepo(repo *repo.Entry) error
	Latest(reponame, chart string) (string, error)
	PullOCI(url string, version string) (string, error)
	Pull(repo string, chart string, version string) (string, error)
	RegistryAuth(server, user, pass string) error
	Push(path, dst string) error
	GetChartMetadata(chartPath string) (*chart.Metadata, error)
	ReleaseExists(ctx context.Context, namespace string, releaseName string) (bool, error)
	Install(ctx context.Context, opts InstallOptions) (*release.Release, error)
	Upgrade(ctx context.Context, opts UpgradeOptions) (*release.Release, error)
	Uninstall(ctx context.Context, opts UninstallOptions) error
	Render(releaseName string, chartPath string, values map[string]interface{}, namespace string, labels map[string]string) ([][]byte, error)
}

// Convenience functions

func NewClient(opts HelmOptions) (Client, error) {
	if _hc != nil {
		return _hc, nil
	}
	return newClient(opts)
}
