package helm

import (
	"context"

	"github.com/stretchr/testify/mock"
	"helm.sh/helm/v3/pkg/chart"
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

func (m *MockClient) AddRepo(ctx context.Context, repo *repo.Entry) error {
	args := m.Called(ctx, repo)
	return args.Error(0)
}

func (m *MockClient) Latest(ctx context.Context, reponame, chart string) (string, error) {
	args := m.Called(ctx, reponame, chart)
	return args.String(0), args.Error(1)
}

func (m *MockClient) Pull(ctx context.Context, reponame, chart string, version string) (string, error) {
	args := m.Called(ctx, reponame, chart, version)
	return args.String(0), args.Error(1)
}

func (m *MockClient) PullByRef(ctx context.Context, ref string, version string) (string, error) {
	args := m.Called(ctx, ref, version)
	return args.String(0), args.Error(1)
}

func (m *MockClient) RegistryAuth(ctx context.Context, server, user, pass string) error {
	args := m.Called(ctx, server, user, pass)
	return args.Error(0)
}

func (m *MockClient) Push(ctx context.Context, path, dst string) error {
	args := m.Called(ctx, path, dst)
	return args.Error(0)
}

func (m *MockClient) GetChartMetadata(ctx context.Context, chartPath string, version string) (*chart.Metadata, error) {
	args := m.Called(ctx, chartPath, version)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*chart.Metadata), args.Error(1)
}

func (m *MockClient) ReleaseExists(ctx context.Context, namespace string, releaseName string) (bool, error) {
	args := m.Called(ctx, namespace, releaseName)
	return args.Bool(0), args.Error(1)
}

func (m *MockClient) Install(ctx context.Context, opts InstallOptions) (string, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return "", args.Error(1)
	}
	return args.Get(0).(string), args.Error(1)
}

func (m *MockClient) Upgrade(ctx context.Context, opts UpgradeOptions) (string, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return "", args.Error(1)
	}
	return args.Get(0).(string), args.Error(1)
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
