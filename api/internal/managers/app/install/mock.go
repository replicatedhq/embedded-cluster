package install

import (
	"context"

	kotscli "github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/mock"
)

// MockAppInstallManager is a mock implementation of the AppInstallManager interface
type MockAppInstallManager struct {
	mock.Mock
}

// Install mocks the Install method
func (m *MockAppInstallManager) Install(ctx context.Context, configValues kotsv1beta1.ConfigValues) error {
	args := m.Called(ctx, configValues)
	return args.Error(0)
}

// MockKotsCLIInstaller is a mock implementation of the KotsCLIInstaller interface
type MockKotsCLIInstaller struct {
	mock.Mock
}

// Install mocks the Install method from the kotscli package
func (m *MockKotsCLIInstaller) Install(opts kotscli.InstallOptions) error {
	args := m.Called(opts)
	return args.Error(0)
}
