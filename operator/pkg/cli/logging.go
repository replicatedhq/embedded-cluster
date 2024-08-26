package cli

import (
	"github.com/bombsimon/logrusr/v4"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
)

// NewLogger creates a new logr.Logger that writes to a logrus.Logger.
func NewLogger(level logrus.Level) (logr.Logger, error) {
	logrusLog := logrus.New()
	logrusLog.SetLevel(level)
	log := logrusr.New(logrusLog)
	return log, nil
}
