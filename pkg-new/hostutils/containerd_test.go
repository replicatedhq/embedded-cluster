package hostutils

import (
	"fmt"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/stretchr/testify/assert"
)

func Test_useContainerdV3Schema(t *testing.T) {
	tests := []struct {
		name       string
		k0sVersion string
		want       bool
	}{
		{name: "unset default", k0sVersion: "0.0.0", want: false},
		{name: "empty", k0sVersion: "", want: false},
		{name: "k0s 1.34", k0sVersion: "v1.34.8+k0s.0", want: false},
		{name: "k0s 1.35", k0sVersion: "v1.35.5+k0s.0", want: false},
		{name: "k0s 1.36", k0sVersion: "v1.36.1+k0s.0", want: true},
		{name: "k0s 1.37", k0sVersion: "v1.37.0+k0s.0", want: true},
	}
	orig := versions.K0sVersion
	t.Cleanup(func() { versions.K0sVersion = orig })
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			versions.K0sVersion = tt.k0sVersion
			assert.Equal(t, tt.want, useContainerdV3Schema())
		})
	}
}

// Test_v2RegistryHostRegex_roundTrip ensures the migration regex can recover the
// registry host from exactly what registryConfigTemplateV2 writes.
func Test_v2RegistryHostRegex_roundTrip(t *testing.T) {
	const host = "10.96.0.10:5000"
	contents := fmt.Sprintf(registryConfigTemplateV2, host)
	match := v2RegistryHostRegex.FindStringSubmatch(contents)
	if assert.NotNil(t, match, "regex should match the v2 template output") {
		assert.Equal(t, host, match[1])
	}

	// The v3 drop-in must NOT match (so migration is a no-op when already v3).
	v3 := fmt.Sprintf(registryConfigTemplateV3, "/etc/k0s/containerd/certs.d")
	assert.Nil(t, v2RegistryHostRegex.FindStringSubmatch(v3), "regex should not match v3 content")
}
