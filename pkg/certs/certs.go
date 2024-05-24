package certs

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

// NewBuilder returns a new certificate builder.
func NewBuilder(opts ...Option) (*Builder, error) {
	builder := &Builder{
		organizations: []string{"replicated"},
		expiration:    time.Now().Add(time.Hour * 24 * 365),
		commonName:    "localhost",
		dnsNames:      []string{"localhost"},
		ipAddresses:   []net.IP{net.ParseIP("127.0.0.1")},
	}
	for _, opt := range opts {
		if err := opt(builder); err != nil {
			return nil, fmt.Errorf("unable to apply option: %w", err)
		}
	}
	return builder, nil
}

// Builder is a helper to generate self signed certificates.
type Builder struct {
	organizations []string
	expiration    time.Time
	commonName    string
	dnsNames      []string
	ipAddresses   []net.IP
	signBy        *SignCA
}

// SignCA holds a path to a key and a certificate we use to sign the new certificate.
type SignCA struct {
	crt []byte
	key []byte
}

// parse reads and parses the CA certificate and key from disk.
func (s *SignCA) parse() (*x509.Certificate, *rsa.PrivateKey, error) {
	caCertBlock, _ := pem.Decode(s.crt)
	if caCertBlock == nil {
		return nil, nil, fmt.Errorf("unable to decode certificate PEM")
	}
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}

	caKeyBlock, _ := pem.Decode(s.key)
	if caKeyBlock == nil {
		return nil, nil, fmt.Errorf("unable to decode key PEM")
	}
	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}

	return caCert, caKey, nil
}

// generatePair creates a new key/crt pair.
func (b *Builder) Generate() (string, string, error) {
	pkey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("unable to generate private key: %w", err)
	}

	kusage := x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature
	tpl := x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		NotBefore:             time.Now(),
		NotAfter:              b.expiration,
		KeyUsage:              kusage,
		BasicConstraintsValid: true,
		IPAddresses:           b.ipAddresses,
		DNSNames:              b.dnsNames,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
		Subject: pkix.Name{
			Organization: b.organizations,
			CommonName:   b.commonName,
		},
	}

	cacert := &tpl
	cakey := pkey
	if b.signBy != nil {
		cacert, cakey, err = b.signBy.parse()
		if err != nil {
			return "", "", fmt.Errorf("unable to parse signer ca: %w", err)
		}
	}

	cert, err := x509.CreateCertificate(rand.Reader, &tpl, cacert, &pkey.PublicKey, cakey)
	if err != nil {
		return "", "", fmt.Errorf("unable to create certificate: %w", err)
	}

	crtbuf := bytes.NewBuffer(nil)
	if err := pem.Encode(
		crtbuf, &pem.Block{Type: "CERTIFICATE", Bytes: cert},
	); err != nil {
		return "", "", fmt.Errorf("unable to encode certificate: %w", err)
	}

	pbytes, err := x509.MarshalPKCS8PrivateKey(pkey)
	if err != nil {
		return "", "", fmt.Errorf("unable to marshal private key: %w", err)
	}

	keybuf := bytes.NewBuffer(nil)
	if err := pem.Encode(
		keybuf, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: pbytes},
	); err != nil {
		return "", "", fmt.Errorf("unable to encode private key: %w", err)
	}

	return crtbuf.String(), keybuf.String(), nil
}
