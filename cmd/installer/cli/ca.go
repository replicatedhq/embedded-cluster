package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
)

// findHostCABundle locates the system CA certificate bundle on the host.
// It first checks the SSL_CERT_FILE environment variable, then searches
// common file paths used by various Linux distributions.
//
// The search follows the same order as the Go standard library's crypto/x509 package.
//
// Returns the path to the first found CA certificate bundle and nil error on success.
// Returns an empty string and error if SSL_CERT_FILE is set but inaccessible
// or if no CA certificate bundle is found.
func findHostCABundle() (string, error) {
	// First check if SSL_CERT_FILE environment variable is set
	if envFile := os.Getenv("SSL_CERT_FILE"); envFile != "" {
		if _, err := helpers.Stat(envFile); err != nil {
			return "", fmt.Errorf("SSL_CERT_FILE set to %s but file cannot be accessed: %w", envFile, err)
		}
		return envFile, nil
	}

	// From https://github.com/golang/go/blob/go1.24.3/src/crypto/x509/root_linux.go
	certFiles := []string{
		"/etc/ssl/certs/ca-certificates.crt",                // Debian/Ubuntu/Gentoo etc.
		"/etc/pki/tls/certs/ca-bundle.crt",                  // Fedora/RHEL 6
		"/etc/ssl/ca-bundle.pem",                            // OpenSUSE
		"/etc/pki/tls/cacert.pem",                           // OpenELEC
		"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem", // CentOS/RHEL 7
		"/etc/ssl/cert.pem",                                 // Alpine Linux
	}

	// Check each file in the order of preference returning the first found
	for _, file := range certFiles {
		// Ignore all errors to replicate the behavior of the Go standard library
		// https://github.com/golang/go/blob/go1.24.3/src/crypto/x509/root_unix.go#L47-L81
		if _, err := helpers.Stat(file); err == nil {
			return file, nil
		}
	}

	return "", errors.New("no CA certificate file found")
}
