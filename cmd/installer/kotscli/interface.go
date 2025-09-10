package kotscli

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

// KotsCLI defines the interface for KOTS CLI operations
type KotsCLI interface {
	Install(opts InstallOptions) error
	ResetPassword(rc runtimeconfig.RuntimeConfig, password string) error
	AirgapUpdate(opts AirgapUpdateOptions) error
	VeleroConfigureOtherS3(opts VeleroConfigureOtherS3Options) error
	GetJoinCommand(ctx context.Context, rc runtimeconfig.RuntimeConfig) (string, error)
	UpstreamUpgrade(opts UpstreamUpgradeOptions) error
	GetVersions(opts GetVersionsOptions) ([]AppVersionResponse, error)
	SetConfig(opts SetConfigOptions) error
	MaskKotsOutputForOnline() spinner.MaskFn
	MaskKotsOutputForAirgap() spinner.MaskFn
}
