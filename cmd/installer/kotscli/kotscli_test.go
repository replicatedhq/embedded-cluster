package kotscli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInstallOptions_TLSFields(t *testing.T) {
	opts := InstallOptions{
		AppSlug:      "test-app",
		Namespace:    "kotsadm",
		TLSCertBytes: []byte("test-cert-data"),
		TLSKeyBytes:  []byte("test-key-data"),
		Hostname:     "test.example.com",
	}

	require.Equal(t, "test-app", opts.AppSlug)
	require.Equal(t, "kotsadm", opts.Namespace)
	require.Equal(t, []byte("test-cert-data"), opts.TLSCertBytes)
	require.Equal(t, []byte("test-key-data"), opts.TLSKeyBytes)
	require.Equal(t, "test.example.com", opts.Hostname)
}

func TestCreateTLSSecret_NoTLSData(t *testing.T) {
	ctx := context.Background()
	opts := InstallOptions{
		Namespace: "kotsadm",
		// No TLS data provided
	}

	// Should return nil without error when no TLS data is provided
	err := createTLSSecret(ctx, opts)
	require.NoError(t, err)
}

func TestCreateTLSSecret_WithTLSData(t *testing.T) {
	ctx := context.Background()
	opts := InstallOptions{
		Namespace:    "kotsadm",
		TLSCertBytes: []byte("test-cert-data"),
		TLSKeyBytes:  []byte("test-key-data"),
		Hostname:     "test.example.com",
	}

	// This test will fail because we can't create a real kubernetes client in unit tests
	// but it verifies the function signature and basic logic
	err := createTLSSecret(ctx, opts)

	// We expect this to fail due to inability to connect to kubernetes in test environment
	require.Error(t, err)
	require.Contains(t, err.Error(), "create kotsadm-tls secret")
}
