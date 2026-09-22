package airgapbundle

import (
	"errors"
	"testing"
)

func TestIsRetryableRegistryStreamError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "http2 internal stream error", err: errors.New("error writing layer: stream error: stream ID 205; INTERNAL_ERROR; received from peer"), want: true},
		{name: "http error", err: errors.New("unexpected status code 500"), want: false},
		{name: "authentication error", err: errors.New("unauthorized"), want: false},
		{name: "nil", err: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableRegistryStreamError(tt.err); got != tt.want {
				t.Fatalf("isRetryableRegistryStreamError() = %t, want %t", got, tt.want)
			}
		})
	}
}
