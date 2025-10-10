package upgrade

import (
	"context"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	addontypes "github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/stretchr/testify/mock"
)

// MockInfraUpgrader is a mock implementation of the InfraUpgrader interface
type MockInfraUpgrader struct {
	mock.Mock
}

func (m *MockInfraUpgrader) CreateInstallation(ctx context.Context, in *ecv1beta1.Installation) error {
	args := m.Called(ctx, in)
	return args.Error(0)
}

func (m *MockInfraUpgrader) CopyVersionMetadataToCluster(ctx context.Context, in *ecv1beta1.Installation) error {
	args := m.Called(ctx, in)
	return args.Error(0)
}

func (m *MockInfraUpgrader) DistributeArtifacts(ctx context.Context, in *ecv1beta1.Installation, localArtifactMirrorImage, licenseID, appSlug, channelID, appVersion string) error {
	args := m.Called(ctx, in, localArtifactMirrorImage, licenseID, appSlug, channelID, appVersion)
	return args.Error(0)
}

func (m *MockInfraUpgrader) UpgradeK0s(ctx context.Context, in *ecv1beta1.Installation) error {
	args := m.Called(ctx, in)
	return args.Error(0)
}

func (m *MockInfraUpgrader) UpdateClusterConfig(ctx context.Context, in *ecv1beta1.Installation) error {
	args := m.Called(ctx, in)
	return args.Error(0)
}

func (m *MockInfraUpgrader) UpgradeAddons(ctx context.Context, in *ecv1beta1.Installation, progressChan chan addontypes.AddOnProgress) error {
	args := m.Called(ctx, in, progressChan)
	return args.Error(0)
}

func (m *MockInfraUpgrader) UpgradeExtensions(ctx context.Context, in *ecv1beta1.Installation) error {
	args := m.Called(ctx, in)
	return args.Error(0)
}

func (m *MockInfraUpgrader) CreateHostSupportBundle(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
