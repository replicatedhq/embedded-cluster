package util

import "testing"

func TestNameWithLengthLimit(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		suffix string
		want   string
	}{
		{
			name:   "short",
			prefix: "short",
			suffix: "short",
			want:   "shortshort",
		},
		{
			name:   "long",
			prefix: "this is not quite 63 characters long",
			suffix: "this is not quite 63 characters long",
			want:   "this is not quite 63 characters is not quite 63 characters long",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NameWithLengthLimit(tt.prefix, tt.suffix); got != tt.want {
				t.Errorf("NameWithLengthLimit() = %v, want %v", got, tt.want)
			}
		})
	}
}
