package main

import (
	"testing"

	"github.com/Masterminds/semver/v3"
)

func TestPackageVersion_matches(t *testing.T) {
	type fields struct {
		semver   semver.Version
		revision int
	}
	type args struct {
		version *semver.Version
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "matches exact",
			fields: fields{
				semver:   *semver.MustParse("1.2.3"),
				revision: 1,
			},
			args: args{
				version: semver.MustParse("1.2.3"),
			},
			want: true,
		},
		{
			name: "not matches exact",
			fields: fields{
				semver:   *semver.MustParse("1.2.3"),
				revision: 1,
			},
			args: args{
				version: semver.MustParse("1.2.4"),
			},
			want: false,
		},
		{
			name: "matches minor",
			fields: fields{
				semver:   *semver.MustParse("1.2.3"),
				revision: 1,
			},
			args: args{
				version: semver.MustParse("1.2"),
			},
			want: true,
		},
		{
			name: "not matches minor",
			fields: fields{
				semver:   *semver.MustParse("1.2.3"),
				revision: 1,
			},
			args: args{
				version: semver.MustParse("1.3"),
			},
			want: false,
		},
		{
			name: "matches major",
			fields: fields{
				semver:   *semver.MustParse("1.2.3"),
				revision: 1,
			},
			args: args{
				version: semver.MustParse("1"),
			},
			want: true,
		},
		{
			name: "not matches major",
			fields: fields{
				semver:   *semver.MustParse("1.2.3"),
				revision: 1,
			},
			args: args{
				version: semver.MustParse("2"),
			},
			want: false,
		},
		{
			name: "ignores prerelease and metadata",
			fields: fields{
				semver:   *semver.MustParse("1.2.3"),
				revision: 1,
			},
			args: args{
				version: semver.MustParse("1.2.3-alpha.1+metadata.2"),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &PackageVersion{
				semver:   tt.fields.semver,
				revision: tt.fields.revision,
			}
			if got := v.matches(tt.args.version); got != tt.want {
				t.Errorf("PackageVersion.matches() = %v, want %v", got, tt.want)
			}
		})
	}
}
