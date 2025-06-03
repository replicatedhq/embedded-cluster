package api

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/sirupsen/logrus"
)

func NewLogger() (*logrus.Logger, error) {
	fname := fmt.Sprintf("%s-%s.api.log", runtimeconfig.BinaryName(), time.Now().Format("20060102150405.000"))
	logpath := runtimeconfig.PathToLog(fname)
	logfile, err := os.OpenFile(logpath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0400)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	logger := logrus.New()
	logger.SetOutput(logfile)

	logger.Infof("versions: embedded-cluster=%s, k0s=%s", versions.Version, versions.K0sVersion)
	logger.Infof("command line arguments: %v", os.Args)

	return logger, nil
}

func NewDiscardLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	return logger
}
