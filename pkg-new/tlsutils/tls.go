package tlsutils

import (
	"crypto/tls"
	"fmt"
	"net"

	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
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

// GetCertificate returns a TLS certificate based on the provided configuration.
// If cert and key files are provided, it uses those. Otherwise, it generates a self-signed certificate.
func GetCertificate(cfg Config) (tls.Certificate, error) {
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		logrus.Debugf("Using TLS configuration with cert file: %s and key file: %s", cfg.CertFile, cfg.KeyFile)
		return tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	}

	hostname, altNames := generateCertHostnames(cfg.Hostname)

	// Generate a new self-signed cert
	certData, keyData, err := certutil.GenerateSelfSignedCertKey(hostname, cfg.IPAddresses, altNames)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate self-signed cert: %w", err)
	}

	cert, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create TLS certificate: %w", err)
	}

	logrus.Debugf("Using self-signed TLS certificate for hostname: %s", hostname)
	return cert, nil
}

// GetTLSConfig returns a TLS configuration with the provided certificate
func GetTLSConfig(cert tls.Certificate) *tls.Config {
	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		CipherSuites: TLSCipherSuites,
		Certificates: []tls.Certificate{cert},
	}
}

func generateCertHostnames(hostname string) (string, []string) {
	namespace := runtimeconfig.KotsadmNamespace

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
