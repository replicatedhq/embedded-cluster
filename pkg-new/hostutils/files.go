package hostutils

import (
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/cmd/installer/goods"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/support"
)

func (h *HostUtils) MaterializeFiles(dataDir string, airgapBundle string) error {
	materializer := goods.NewMaterializer(dataDir)
	if err := materializer.Materialize(); err != nil {
		return fmt.Errorf("materialize binaries: %w", err)
	}
	if err := support.MaterializeSupportBundleSpec(dataDir); err != nil {
		return fmt.Errorf("materialize support bundle spec: %w", err)
	}

	if airgapBundle != "" {
		// read file from path
		rawfile, err := os.Open(airgapBundle)
		if err != nil {
			return fmt.Errorf("failed to open airgap file: %w", err)
		}
		defer rawfile.Close()

		if err := airgap.MaterializeAirgap(dataDir, rawfile); err != nil {
			return fmt.Errorf("materialize airgap files: %w", err)
		}
	}

	return nil
}
