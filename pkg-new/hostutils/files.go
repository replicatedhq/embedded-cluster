package hostutils

import (
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/cmd/installer/goods"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/support"
)

func (h *HostUtils) MaterializeFiles(rc runtimeconfig.RuntimeConfig, airgapBundle string) error {
	materializer := goods.NewMaterializer(rc)
	if err := materializer.Materialize(); err != nil {
		return fmt.Errorf("materialize binaries: %w", err)
	}

	isAirgap := airgapBundle != ""
	if err := support.MaterializeSupportBundleSpec(rc, isAirgap); err != nil {
		return fmt.Errorf("materialize support bundle spec: %w", err)
	}

	if airgapBundle != "" {
		// read file from path
		rawfile, err := os.Open(airgapBundle)
		if err != nil {
			return fmt.Errorf("failed to open airgap file: %w", err)
		}
		defer rawfile.Close()

		if err := airgap.MaterializeAirgap(rc, rawfile); err != nil {
			return fmt.Errorf("materialize airgap files: %w", err)
		}
	}

	return nil
}
