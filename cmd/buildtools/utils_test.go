package main

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPackageVersions_Sort(t *testing.T) {
	tests := []struct {
		name string
		pvs  []*PackageVersion
		want PackageVersions
	}{
		{
			name: "basic",
			pvs: []*PackageVersion{
				MustParsePackageVersion("1.0.1-r1"),
				MustParsePackageVersion("1.0.1-r3"),
				MustParsePackageVersion("1.0.1-r2"),
				MustParsePackageVersion("1.0.1-r0"),
				MustParsePackageVersion("1.2.0-r1"),
				MustParsePackageVersion("1.2.0-r3"),
				MustParsePackageVersion("1.2.0-r2"),
				MustParsePackageVersion("1.2.0-r0"),
				MustParsePackageVersion("1.0.2-r1"),
				MustParsePackageVersion("1.0.2-r3"),
				MustParsePackageVersion("1.0.2-r2"),
				MustParsePackageVersion("1.0.2-r0"),
			},
			want: PackageVersions{
				MustParsePackageVersion("1.0.1-r0"),
				MustParsePackageVersion("1.0.1-r1"),
				MustParsePackageVersion("1.0.1-r2"),
				MustParsePackageVersion("1.0.1-r3"),
				MustParsePackageVersion("1.0.2-r0"),
				MustParsePackageVersion("1.0.2-r1"),
				MustParsePackageVersion("1.0.2-r2"),
				MustParsePackageVersion("1.0.2-r3"),
				MustParsePackageVersion("1.2.0-r0"),
				MustParsePackageVersion("1.2.0-r1"),
				MustParsePackageVersion("1.2.0-r2"),
				MustParsePackageVersion("1.2.0-r3"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sorted := PackageVersions(tt.pvs)
			sort.Sort(sorted)
			assert.Equal(t, tt.want, sorted)
		})
	}
}
