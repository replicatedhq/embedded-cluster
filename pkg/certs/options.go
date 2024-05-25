package certs

import (
	"fmt"
	"net"
	"os"
	"time"
)

// Option is a function that applies a configuration option to a Builder.
type Option func(*Builder) error

// WithExpiration sets the expiration date for the certificate.
func WithExpiration(expiration time.Time) Option {
	return func(b *Builder) error {
		b.expiration = expiration
		return nil
	}
}

// WithDuration sets the duration for the certificate from now.
func WithDuration(d time.Duration) Option {
	return func(b *Builder) error {
		b.expiration = time.Now().Add(d)
		return nil
	}
}

// WithOrganization sets the organization in the certificate.
func WithOrganization(org string) Option {
	return func(b *Builder) error {
		b.organizations = append(b.organizations, org)
		return nil
	}
}

// WithCommonName sets the common name in the certificate.
func WithCommonName(cn string) Option {
	return func(b *Builder) error {
		b.commonName = cn
		return nil
	}
}

// WithIPAddress adds an IP address to the certificate.
func WithIPAddress(ip string) Option {
	return func(b *Builder) error {
		b.ipAddresses = append(b.ipAddresses, net.ParseIP(ip))
		return nil
	}
}

// WithDNSName adds a DNS name to the certificate.
func WithDNSName(name string) Option {
	return func(b *Builder) error {
		b.dnsNames = append(b.dnsNames, name)
		return nil
	}
}

// SignWithDiskFiles allows to sign the certificate with the CA certificate and key from disk.
func SignWithDiskFiles(certPath, keyPath string) Option {
	return func(b *Builder) error {
		crt, err := os.ReadFile(certPath)
		if err != nil {
			return fmt.Errorf("unable to read cert file: %w", err)
		}
		key, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("unable to read key file: %w", err)
		}
		b.signBy = &SignCA{crt: crt, key: key}
		return nil
	}
}

// SignWith sets the CA to sign the certificate.
func SignWith(certPath, keyPath []byte) Option {
	return func(b *Builder) error {
		b.signBy = &SignCA{
			crt: certPath,
			key: keyPath,
		}
		return nil
	}
}
