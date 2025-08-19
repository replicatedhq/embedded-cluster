package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Integration test to verify TLS flags are no longer hidden
func TestTLSFlagsNotHidden(t *testing.T) {
	cmd := InstallCmd(nil, "test-app", "Test App")
	require.NotNil(t, cmd)

	// Check that TLS flags exist and are not hidden
	tlsCertFlag := cmd.Flags().Lookup("tls-cert")
	require.NotNil(t, tlsCertFlag, "tls-cert flag should exist")
	require.False(t, tlsCertFlag.Hidden, "tls-cert flag should not be hidden")

	tlsKeyFlag := cmd.Flags().Lookup("tls-key")
	require.NotNil(t, tlsKeyFlag, "tls-key flag should exist")
	require.False(t, tlsKeyFlag.Hidden, "tls-key flag should not be hidden")

	hostnameFlag := cmd.Flags().Lookup("hostname")
	require.NotNil(t, hostnameFlag, "hostname flag should exist")
	require.False(t, hostnameFlag.Hidden, "hostname flag should not be hidden")
}

func TestTLSFlagDescriptions(t *testing.T) {
	cmd := InstallCmd(nil, "test-app", "Test App")
	require.NotNil(t, cmd)

	tlsCertFlag := cmd.Flags().Lookup("tls-cert")
	require.Equal(t, "Path to the TLS certificate file", tlsCertFlag.Usage)

	tlsKeyFlag := cmd.Flags().Lookup("tls-key")
	require.Equal(t, "Path to the TLS key file", tlsKeyFlag.Usage)

	hostnameFlag := cmd.Flags().Lookup("hostname")
	require.Equal(t, "Hostname to use for TLS configuration", hostnameFlag.Usage)
}

func TestInstallCommandStructure(t *testing.T) {
	cmd := InstallCmd(nil, "test-app", "Test App")
	require.NotNil(t, cmd)
	require.Equal(t, "install", cmd.Use)
	require.Contains(t, cmd.Short, "Test App")
}
