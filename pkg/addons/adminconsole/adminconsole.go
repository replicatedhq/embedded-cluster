package adminconsole

import (
	"fmt"
	"os"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	_releaseName = "admin-console"
)

var _ types.AddOn = (*AdminConsole)(nil)

type AdminConsole struct {
	IsAirgap           bool
	IsHA               bool
	IsMultiNodeEnabled bool
	Proxy              *ecv1beta1.ProxySpec
	AdminConsolePort   int

	// Linux specific options
	ClusterID        string
	ServiceCIDR      string
	HostCABundlePath string
	DataDir          string
	K0sDataDir       string

	// These options are only used during installation
	Password         string
	TLSCertBytes     []byte
	TLSKeyBytes      []byte
	Hostname         string
	KotsInstaller    KotsInstaller
	KotsadmNamespace string

	// DryRun is a flag to enable dry-run mode for Admin Console.
	// If true, Admin Console will only render the helm template and additional manifests, but not install
	// the release.
	DryRun bool

	dryRunManifests [][]byte
}

type KotsInstaller func() error

func (a *AdminConsole) Name() string {
	return "Admin Console"
}

func (a *AdminConsole) Version() string {
	return Metadata.Version
}

func (a *AdminConsole) ReleaseName() string {
	return _releaseName
}

func (a *AdminConsole) Namespace() string {
	return a.KotsadmNamespace
}

func getBackupLabels() map[string]string {
	return map[string]string{
		"replicated.com/disaster-recovery":       "infra",
		"replicated.com/disaster-recovery-chart": "admin-console",
	}
}

// TLSSecretName returns the name of the kotsadm TLS secret.
func TLSSecretName() string {
	return "kotsadm-tls"
}

// NewTLSSecret creates a new kotsadm-tls secret with the given certificate, key, and hostname.
func NewTLSSecret(namespace string, certBytes, keyBytes []byte, hostname string) *corev1.Secret {
	labels := getBackupLabels()
	labels["kots.io/kotsadm"] = "true"

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      TLSSecretName(),
			Namespace: namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"acceptAnonymousUploads": "0",
			},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": certBytes,
			"tls.key": keyBytes,
		},
	}
	if hostname != "" {
		secret.StringData = map[string]string{"hostname": hostname}
	}
	return secret
}

func (a *AdminConsole) ChartLocation(domains ecv1beta1.Domains) string {
	chartName := Metadata.Location
	if AdminConsoleChartRepoOverride != "" {
		chartName = fmt.Sprintf("oci://proxy.replicated.com/anonymous/%s", AdminConsoleChartRepoOverride)
	}

	if domains.ProxyRegistryDomain != "" {
		chartName = strings.Replace(chartName, "proxy.replicated.com", domains.ProxyRegistryDomain, 1)
	}
	return chartName
}

// DryRunManifests returns the manifests generated during a dry run.
func (a *AdminConsole) DryRunManifests() [][]byte {
	return a.dryRunManifests
}

func (a *AdminConsole) isEmbeddedCluster() bool {
	return a.ClusterID != ""
}

func (a *AdminConsole) isV3() bool {
	return os.Getenv("ENABLE_V3") == "1"
}
