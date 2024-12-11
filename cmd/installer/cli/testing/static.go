package testing

import (
	"embed"
)

var (
	//go:embed assets/release-restore-legacydr
	RestoreReleaseDataLegacyDR embed.FS

	//go:embed assets/release-restore-newdr
	RestoreReleaseDataNewDR embed.FS
)
