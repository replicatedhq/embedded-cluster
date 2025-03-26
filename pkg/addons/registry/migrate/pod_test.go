package migrate

import (
	"bytes"
	"log/slog"
	"testing"
)

func Test_isProgressLogLine(t *testing.T) {
	buf := new(bytes.Buffer)
	logger := slog.New(slog.NewTextHandler(buf, nil))

	logger.Info("test", getProgressArgs(2, 10)...)
	if !isProgressLogLine(buf.String()) {
		t.Errorf("isProgressLogLine(%v) = false, want true", buf.String())
	}
	buf.Reset()

	logger.Info("not a progress line")
	if isProgressLogLine(buf.String()) {
		t.Errorf("isProgressLogLine(%v) = true, want false", buf.String())
	}
	buf.Reset()
}

func Test_getProgressFromLogLine(t *testing.T) {
	buf := new(bytes.Buffer)
	logger := slog.New(slog.NewTextHandler(buf, nil))

	logger.Info("test", getProgressArgs(2, 10)...)
	if got := getProgressFromLogLine(buf.String()); got != "20%" {
		t.Errorf("getProgressFromLogLine(%v) = %v, want %v", buf.String(), got, "20%")
	}
	buf.Reset()

	logger.Info("not a progress line")
	if got := getProgressFromLogLine(buf.String()); got != "" {
		t.Errorf("getProgressFromLogLine(%v) = %v, want %v", buf.String(), got, "")
	}
	buf.Reset()
}
