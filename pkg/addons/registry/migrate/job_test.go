package migrate

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
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

func Test_isJobRetrying(t *testing.T) {
	type args struct {
		ctx context.Context
		cli client.Client
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := isJobRetrying(tt.args.ctx, tt.args.cli)
			if (err != nil) != tt.wantErr {
				t.Errorf("isJobRetrying() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("isJobRetrying() = %v, want %v", got, tt.want)
			}
		})
	}
}
