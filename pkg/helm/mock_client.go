package helm

import (
	"context"

	"github.com/stretchr/testify/mock"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
)

var _ Client = (*MockClient)(nil)

type MockClient struct {
	mock.Mock
}

func (m *MockClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockClient) AddRepo(repo *repo.Entry) error {
	args := m.Called(repo)
	return args.Error(0)
}

func (m *MockClient) Latest(reponame, chart string) (string, error) {
	args := m.Called(reponame, chart)
	return args.String(0), args.Error(1)
}

func (m *MockClient) Pull(reponame, chart string, version string) (string, error) {
	args := m.Called(reponame, chart, version)
	return args.String(0), args.Error(1)
}

func (m *MockClient) PullByRef(ref string, version string) (string, error) {
	args := m.Called(ref, version)
	return args.String(0), args.Error(1)
}

func (m *MockClient) RegistryAuth(server, user, pass string) error {
	args := m.Called(server, user, pass)
	return args.Error(0)
}

func (m *MockClient) Push(path, dst string) error {
	args := m.Called(path, dst)
	return args.Error(0)
}

func (m *MockClient) GetChartMetadata(chartPath string) (*chart.Metadata, error) {
	args := m.Called(chartPath)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*chart.Metadata), args.Error(1)
}

func (m *MockClient) ReleaseExists(ctx context.Context, namespace string, releaseName string) (bool, error) {
	args := m.Called(ctx, namespace, releaseName)
	return args.Bool(0), args.Error(1)
}

func (m *MockClient) Install(ctx context.Context, opts InstallOptions) (*release.Release, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*release.Release), args.Error(1)
}

func (m *MockClient) Upgrade(ctx context.Context, opts InstallOptions) (*release.Release, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*release.Release), args.Error(1)
}

func (m *MockClient) Uninstall(ctx context.Context, opts UninstallOptions) error {
	args := m.Called(ctx, opts)
	return args.Error(0)
}

func (m *MockClient) Render(ctx context.Context, opts InstallOptions) ([][]byte, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([][]byte), args.Error(1)
}
