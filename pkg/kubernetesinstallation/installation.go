package kubernetesinstallation

import (
	"os"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
)

var _ Installation = &kubernetesInstallation{}

type Option func(*kubernetesInstallation)

type EnvSetter interface {
	Setenv(key string, val string) error
}

type kubernetesInstallation struct {
	installation *ecv1beta1.KubernetesInstallation
	envSetter    EnvSetter
}

type osEnvSetter struct{}

func (o *osEnvSetter) Setenv(key string, val string) error {
	return os.Setenv(key, val)
}

func WithEnvSetter(envSetter EnvSetter) Option {
	return func(rc *kubernetesInstallation) {
		rc.envSetter = envSetter
	}
}

// New creates a new KubernetesInstallation instance
func New(installation *ecv1beta1.KubernetesInstallation, opts ...Option) Installation {
	if installation == nil {
		installation = &ecv1beta1.KubernetesInstallation{
			Spec: ecv1beta1.GetDefaultKubernetesInstallationSpec(),
		}
	}

	ki := &kubernetesInstallation{installation: installation}
	for _, opt := range opts {
		opt(ki)
	}

	if ki.envSetter == nil {
		ki.envSetter = &osEnvSetter{}
	}

	return ki
}

// Get returns the KubernetesInstallation.
func (ki *kubernetesInstallation) Get() *ecv1beta1.KubernetesInstallation {
	return ki.installation
}

// Set sets the KubernetesInstallation.
func (ki *kubernetesInstallation) Set(installation *ecv1beta1.KubernetesInstallation) {
	if installation == nil {
		return
	}
	ki.installation = installation
}

// SetStatus sets the status for the KubernetesInstallation.
func (ki *kubernetesInstallation) SetStatus(status ecv1beta1.KubernetesInstallationStatus) {
	ki.installation.SetStatus(status)
}

// GetStatus returns the status for the KubernetesInstallation.
func (ki *kubernetesInstallation) GetStatus() ecv1beta1.KubernetesInstallationStatus {
	return ki.installation.Status
}

// SetEnv sets the environment variables for the KubernetesInstallation.
func (ki *kubernetesInstallation) SetEnv() error {
	return nil
}

// AdminConsolePort returns the configured port for the admin console or the default if not
// configured.
func (ki *kubernetesInstallation) AdminConsolePort() int {
	if ki.installation.Spec.AdminConsole.Port > 0 {
		return ki.installation.Spec.AdminConsole.Port
	}
	return ecv1beta1.DefaultAdminConsolePort
}

// ManagerPort returns the configured port for the manager or the default if not
// configured.
func (ki *kubernetesInstallation) ManagerPort() int {
	if ki.installation.Spec.Manager.Port > 0 {
		return ki.installation.Spec.Manager.Port
	}
	return ecv1beta1.DefaultManagerPort
}

// ProxySpec returns the configured proxy spec or nil if not configured.
func (ki *kubernetesInstallation) ProxySpec() *ecv1beta1.ProxySpec {
	return ki.installation.Spec.Proxy
}

// SetAdminConsolePort sets the port for the admin console.
func (ki *kubernetesInstallation) SetAdminConsolePort(port int) {
	ki.installation.Spec.AdminConsole.Port = port
}

// SetManagerPort sets the port for the manager.
func (ki *kubernetesInstallation) SetManagerPort(port int) {
	ki.installation.Spec.Manager.Port = port
}

// SetProxySpec sets the proxy spec for the kubernetes installation.
func (ki *kubernetesInstallation) SetProxySpec(proxySpec *ecv1beta1.ProxySpec) {
	ki.installation.Spec.Proxy = proxySpec
}
