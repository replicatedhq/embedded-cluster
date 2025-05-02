package kubeutils

import (
	"testing"

	"github.com/Masterminds/semver/v3"
)

func Test_lessThanECVersion115(t *testing.T) {
	type args struct {
		ver *semver.Version
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "less than 1.15",
			args: args{
				ver: semver.MustParse("1.14.0+k8s-1.29-49-gf92daca6"),
			},
			want: true,
		},
		{
			name: "greater than or equal to 1.15",
			args: args{
				ver: semver.MustParse("1.15.0+k8s-1.29-49-gf92daca6"),
			},
			want: false,
		},
		{
			name: "old version scheme",
			args: args{
				ver: semver.MustParse("1.28.7+ec.0"),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lessThanECVersion115(tt.args.ver); got != tt.want {
				t.Errorf("lessThanECVersion115() = %v, want %v", got, tt.want)
			}
		})
	}
}
