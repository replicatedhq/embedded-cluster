package testutils

import (
	"testing"

	"github.com/sirupsen/logrus"
)

type logWriter struct{ t *testing.T }

func (l logWriter) Write(p []byte) (n int, err error) {
	l.t.Log(string(p))
	return len(p), nil
}

func TestLogger(t *testing.T) *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(logWriter{t: t})
	return logger
}
