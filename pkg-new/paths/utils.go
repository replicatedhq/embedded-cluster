package paths

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

func InitDataDir(dataDir string, logger logrus.FieldLogger) error {
	logger.Debugf("ensuring data dir exists: %s", dataDir)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	logger.Debugf("ensuring data dir has correct permissions")
	if err := os.Chmod(dataDir, 0755); err != nil {
		logger.Debugf("unable to chmod data dir: %w", err)
	}

	logger.Debugf("ensuring data dir subdirs exist")
	subDirs := []string{
		TmpSubDir(dataDir),
		BinsSubDir(dataDir),
		ChartsSubDir(dataDir),
		ImagesSubDir(dataDir),
		K0sSubDir(dataDir),
		SeaweedfsSubDir(dataDir),
		OpenEBSLocalSubDir(dataDir),
		SupportSubDir(dataDir),
	}
	for _, subDir := range subDirs {
		if err := os.MkdirAll(subDir, 0755); err != nil {
			return fmt.Errorf("create subdir: %w", err)
		}
	}

	return nil
}
