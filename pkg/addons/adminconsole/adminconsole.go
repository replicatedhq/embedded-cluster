// Package adminconsole manages the Kots Admin Console helm chart installation
// or upgrade in the cluster.
package adminconsole

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	eckinds "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

const (
	releaseName = "admin-console"
)

// Overwritten by -ldflags in Makefile
var (
	ChartURL                = "https://url"
	ChartName               = "name"
	Version                 = "v0.0.0"
	ImageOverride           = ""
	MigrationsImageOverride = ""
	CounterRegex            = regexp.MustCompile(`(\d+)/(\d+)`)
	Password                = ""
)

// protectedFields are helm values that are not overwritten when upgrading the addon.
var protectedFields = []string{"automation", "embeddedClusterID", "isAirgap"}

const DEFAULT_ADMIN_CONSOLE_NODE_PORT = 30000

var helmValues = map[string]interface{}{
	"isHA":          false,
	"minimalRBAC":   false,
	"isHelmManaged": false,
	"service": map[string]interface{}{
		"enabled": false, // disable the admin console service
	},
	"kurlProxy": map[string]interface{}{
		"enabled":  true,
		"nodePort": DEFAULT_ADMIN_CONSOLE_NODE_PORT,
	},
	"embeddedClusterVersion": defaults.Version,
	"labels": map[string]interface{}{
		"replicated.com/disaster-recovery":       "infra",
		"replicated.com/disaster-recovery-chart": "admin-console",
	},
	"passwordSecretRef": map[string]interface{}{
		"name": "kotsadm-password",
		"key":  "passwordBcrypt",
	},
}

func init() {
	if ImageOverride != "" {
		helmValues["images"] = map[string]interface{}{
			"kotsadm": ImageOverride,
		}
	}
	if MigrationsImageOverride != "" {
		if helmValues["images"] == nil {
			helmValues["images"] = map[string]interface{}{}
		}
		helmValues["images"].(map[string]interface{})["migrations"] = MigrationsImageOverride
	}
}

// AdminConsole manages the admin console helm chart installation.
type AdminConsole struct {
	namespace    string
	useprompt    bool
	config       v1beta1.ClusterConfig
	licenseFile  string
	airgapBundle string
	proxyEnv     map[string]string
}

func (a *AdminConsole) askPassword() (string, error) {
	defaultPass := "password"
	if !a.useprompt {
		logrus.Info("Admin Console password set to: password")
		return defaultPass, nil
	}
	maxTries := 3
	for i := 0; i < maxTries; i++ {
		promptA := prompts.New().Password("Enter an Admin Console password:")
		promptB := prompts.New().Password("Confirm password:")

		if promptA == promptB {
			return promptA, nil
		}
		logrus.Info("Passwords don't match, please try again.")
	}
	return "", fmt.Errorf("unable to set Admin Console password after %d tries", maxTries)
}

// Version returns the embedded admin console version.
func (a *AdminConsole) Version() (map[string]string, error) {
	return map[string]string{"AdminConsole": "v" + Version}, nil
}

func (a *AdminConsole) Name() string {
	return "AdminConsole"
}

// GetProtectedFields returns the helm values that are not overwritten when upgrading
func (a *AdminConsole) GetProtectedFields() map[string][]string {
	return map[string][]string{releaseName: protectedFields}
}

// HostPreflights returns the host preflight objects found inside the adminconsole
// or as part of the embedded kots release.
func (a *AdminConsole) HostPreflights() (*v1beta2.HostPreflightSpec, error) {
	return release.GetHostPreflights()
}

// GetCurrentChartConfig returns the current adminconsole chart config from the cluster config.
func (a *AdminConsole) GetCurrentChartConfig() *v1beta1.Chart {
	if a.config.Spec == nil || a.config.Spec.Extensions == nil {
		return nil
	}
	if a.config.Spec.Extensions.Helm == nil {
		return nil
	}
	chtlist := a.config.Spec.Extensions.Helm.Charts
	for _, chart := range chtlist {
		if chart.Name == releaseName {
			return &chart
		}
	}
	return nil
}

// GenerateHelmConfig generates the helm config for the adminconsole and writes the charts to
// the disk.
func (a *AdminConsole) GenerateHelmConfig(onlyDefaults bool) ([]eckinds.Chart, []v1beta1.Repository, error) {
	if !onlyDefaults {
		if Password == "" {
			var err error
			Password, err = a.askPassword()
			if err != nil {
				return nil, nil, fmt.Errorf("unable to set kotsadm-password: %w", err)
			}
		}
		helmValues["embeddedClusterID"] = metrics.ClusterID().String()
		if a.airgapBundle != "" {
			helmValues["isAirgap"] = "true"
		} else {
			helmValues["isAirgap"] = "false"
		}
		if len(a.proxyEnv) > 0 {
			extraEnv := []map[string]interface{}{}
			for k, v := range a.proxyEnv {
				extraEnv = append(extraEnv, map[string]interface{}{
					"name":  k,
					"value": v,
				})
			}
			helmValues["extraEnv"] = extraEnv
		}
	}
	values, err := yaml.Marshal(helmValues)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal helm values: %w", err)
	}
	chartConfig := eckinds.Chart{
		Name:      releaseName,
		ChartName: fmt.Sprintf("%s/%s", ChartURL, ChartName),
		Version:   Version,
		Values:    string(values),
		TargetNS:  a.namespace,
		Order:     5,
	}
	return []eckinds.Chart{chartConfig}, nil, nil
}

func (a *AdminConsole) GetAdditionalImages() []string {
	return nil
}

// Outro waits for the adminconsole to be ready.
func (a *AdminConsole) Outro(ctx context.Context, cli client.Client) error {
	loading := spinner.Start()
	loading.Infof("Waiting for the Admin Console to deploy")
	defer loading.Close()

	if err := createKotsPasswordSecret(ctx, cli, a.namespace, Password); err != nil {
		return fmt.Errorf("unable to create kots password secret: %w", err)
	}

	if a.airgapBundle != "" {
		err := createRegistrySecret(ctx, cli, a.namespace)
		if err != nil {
			return fmt.Errorf("error creating registry secret: %v", err)
		}
	}

	if err := WaitForReady(ctx, cli, a.namespace, loading); err != nil {
		return err
	}

	if a.licenseFile != "" {
		license, err := helpers.ParseLicense(a.licenseFile)
		if err != nil {
			return fmt.Errorf("unable to parse license: %w", err)
		}
		installOpts := kotscli.InstallOptions{
			AppSlug:      license.Spec.AppSlug,
			LicenseFile:  a.licenseFile,
			Namespace:    a.namespace,
			AirgapBundle: a.airgapBundle,
		}
		if err := kotscli.Install(installOpts, loading); err != nil {
			return err
		}
	}

	loading.Infof("Admin Console is ready!")

	return nil
}

// New creates a new AdminConsole object.
func New(ns string, useprompt bool, config v1beta1.ClusterConfig, licenseFile string, airgapBundle string, proxyEnv map[string]string) (*AdminConsole, error) {
	return &AdminConsole{
		namespace:    ns,
		useprompt:    useprompt,
		config:       config,
		licenseFile:  licenseFile,
		airgapBundle: airgapBundle,
		proxyEnv:     proxyEnv,
	}, nil
}

// WaitForReady waits for the admin console to be ready.
func WaitForReady(ctx context.Context, cli client.Client, ns string, writer *spinner.MessageWriter) error {
	backoff := wait.Backoff{Steps: 60, Duration: 5 * time.Second, Factor: 1.0, Jitter: 0.1}
	var lasterr error
	if err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		var count int
		ready, err := kubeutils.IsDeploymentReady(ctx, cli, ns, "kotsadm")
		if err != nil {
			lasterr = fmt.Errorf("error checking status of kotsadm: %v", err)
			return false, nil
		}
		if ready {
			count++
		}
		ready, err = kubeutils.IsStatefulSetReady(ctx, cli, ns, "kotsadm-rqlite")
		if err != nil {
			lasterr = fmt.Errorf("error checking status of kotsadm-rqlite: %v", err)
			return false, nil
		}
		if ready {
			count++
		}
		if writer != nil {
			writer.Infof("Waiting for the Admin Console to deploy: %d/2 ready", count)
		}
		return count == 2, nil
	}); err != nil {
		if lasterr == nil {
			lasterr = err
		}
		return fmt.Errorf("error waiting for admin console: %v", lasterr)
	}
	return nil
}

// GetURL returns the URL to the admin console.
func GetURL() string {
	ipaddr := defaults.TryDiscoverPublicIP()
	if ipaddr == "" {
		var err error
		ipaddr, err = defaults.PreferredNodeIPAddress()
		if err != nil {
			logrus.Errorf("unable to determine node IP address: %v", err)
			ipaddr = "NODE-IP-ADDRESS"
		}
	}
	return fmt.Sprintf("http://%s:%v", ipaddr, DEFAULT_ADMIN_CONSOLE_NODE_PORT)
}

func createRegistrySecret(ctx context.Context, cli client.Client, namespace string) error {
	if err := kubeutils.WaitForNamespace(ctx, cli, namespace); err != nil {
		return err
	}

	authString := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("embedded-cluster:%s", registry.GetRegistryPassword())))
	authConfig := fmt.Sprintf(`{"auths":{"%s:5000":{"username": "embedded-cluster", "password": "%s", "auth": "%s"}}}`, registry.GetRegistryClusterIP(), registry.GetRegistryPassword(), authString)

	registryCreds := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-creds",
			Namespace: namespace,
			Labels: map[string]string{
				"kots.io/kotsadm":                        "true",
				"replicated.com/disaster-recovery":       "infra",
				"replicated.com/disaster-recovery-chart": "admin-console",
			},
		},
		StringData: map[string]string{
			".dockerconfigjson": authConfig,
		},
		Type: "kubernetes.io/dockerconfigjson",
	}

	err := cli.Create(ctx, &registryCreds)
	if err != nil {
		return fmt.Errorf("unable to create registry-auth secret: %w", err)
	}

	return nil
}

func createKotsPasswordSecret(ctx context.Context, cli client.Client, namespace string, password string) error {
	if err := kubeutils.WaitForNamespace(ctx, cli, namespace); err != nil {
		return err
	}

	passwordBcrypt, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		return fmt.Errorf("unable to generate bcrypt from password: %w", err)
	}

	kotsPasswordSecret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kotsadm-password",
			Namespace: namespace,
			Labels: map[string]string{
				"kots.io/kotsadm":                        "true",
				"replicated.com/disaster-recovery":       "infra",
				"replicated.com/disaster-recovery-chart": "admin-console",
			},
		},
		Data: map[string][]byte{
			"passwordBcrypt": []byte(passwordBcrypt),
		},
	}

	err = cli.Create(ctx, &kotsPasswordSecret)
	if err != nil {
		return fmt.Errorf("unable to create kotsadm-password secret: %w", err)
	}

	return nil
}
