package tlsutils

import (
	"crypto/tls"
	"fmt"
	"net"

	certutil "k8s.io/client-go/util/cert"
)

var (
	// TLSCipherSuites defines the allowed cipher suites for TLS connections
	TLSCipherSuites = []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
	}
)

// Config represents TLS configuration options
type Config struct {
	CertFile    string
	KeyFile     string
	Hostname    string
	IPAddresses []net.IP
}

// GenerateCertificate creates a new self-signed TLS certificate
func GenerateCertificate(hostname string, ipAddresses []net.IP, namespace string) (tls.Certificate, []byte, []byte, error) {
	hostname, altNames := generateCertHostnames(hostname, namespace)

	// Generate a new self-signed cert
	certData, keyData, err := certutil.GenerateSelfSignedCertKey(hostname, ipAddresses, altNames)
	if err != nil {
		return tls.Certificate{}, nil, nil, fmt.Errorf("generate self-signed cert: %w", err)
	}

	cert, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		return tls.Certificate{}, nil, nil, fmt.Errorf("create TLS certificate: %w", err)
	}

	return cert, certData, keyData, nil
}

// GetTLSConfig returns a TLS configuration with the provided certificate
func GetTLSConfig(cert tls.Certificate) *tls.Config {
	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		CipherSuites: TLSCipherSuites,
		Certificates: []tls.Certificate{cert},
	}
}

func generateCertHostnames(hostname string, namespace string) (string, []string) {

	if hostname == "" {
		hostname = fmt.Sprintf("kotsadm.%s.svc.cluster.local", namespace)
	}

	altNames := []string{
		"kotsadm",
		fmt.Sprintf("kotsadm.%s", namespace),
		fmt.Sprintf("kotsadm.%s.svc", namespace),
		fmt.Sprintf("kotsadm.%s.svc.cluster", namespace),
		fmt.Sprintf("kotsadm.%s.svc.cluster.local", namespace),
	}

	return hostname, altNames
}
