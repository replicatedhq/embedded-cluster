// Package adminconsole manages the Kots Admin Console helm chart installation
// or upgrade in the cluster.
package adminconsole

import (
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"time"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ReleaseName = "admin-console"
)

var (
	//go:embed static/values.tpl.yaml
	rawvalues []byte
	// helmValues is the unmarshal version of rawvalues.
	helmValues map[string]interface{}
	//go:embed static/metadata.yaml
	rawmetadata []byte
	// Metadata is the unmarshal version of rawmetadata.
	Metadata release.AddonMetadata
	// protectedFields are helm values that are not overwritten when upgrading the addon.
	protectedFields = []string{"automation", "embeddedClusterID", "isAirgap"}
	// Overwritten by -ldflags in Makefile
	AdminConsoleChartRepoOverride       = ""
	AdminConsoleImageOverride           = ""
	AdminConsoleMigrationsImageOverride = ""
	AdminConsoleKurlProxyImageOverride  = ""
	KotsVersion                         = ""
)

func init() {
	if err := yaml.Unmarshal(rawmetadata, &Metadata); err != nil {
		panic(fmt.Sprintf("unable to unmarshal metadata: %v", err))
	}
	Render()
}

func Render() {
	hv, err := release.RenderHelmValues(rawvalues, Metadata)
	if err != nil {
		panic(fmt.Sprintf("unable to unmarshal values: %v", err))
	}
	helmValues = hv

	helmValues["embeddedClusterVersion"] = versions.Version

	if AdminConsoleImageOverride != "" {
		helmValues["images"].(map[string]interface{})["kotsadm"] = AdminConsoleImageOverride
	}
	if AdminConsoleMigrationsImageOverride != "" {
		helmValues["images"].(map[string]interface{})["migrations"] = AdminConsoleMigrationsImageOverride
	}
	if AdminConsoleKurlProxyImageOverride != "" {
		helmValues["images"].(map[string]interface{})["kurlProxy"] = AdminConsoleKurlProxyImageOverride
	}
}

// AdminConsole manages the admin console helm chart installation.
type AdminConsole struct {
	namespace        string
	password         string
	licenseFile      string
	airgapBundle     string
	isAirgap         bool
	isHA             bool
	proxyEnv         map[string]string
	privateCAs       map[string]string
	configValuesFile string
}

// Version returns the embedded admin console version.
func (a *AdminConsole) Version() (map[string]string, error) {
	return map[string]string{"AdminConsole": "v" + Metadata.Version}, nil
}

func (a *AdminConsole) Name() string {
	return "AdminConsole"
}

// GetProtectedFields returns the helm values that are not overwritten when upgrading
func (a *AdminConsole) GetProtectedFields() map[string][]string {
	return map[string][]string{ReleaseName: protectedFields}
}

// HostPreflights returns the host preflight objects found inside the adminconsole
// or as part of the embedded kots release.
func (a *AdminConsole) HostPreflights() (*v1beta2.HostPreflightSpec, error) {
	return release.GetHostPreflights()
}

// GenerateHelmConfig generates the helm config for the adminconsole and writes the charts to
// the disk.
func (a *AdminConsole) GenerateHelmConfig(k0sCfg *k0sv1beta1.ClusterConfig, onlyDefaults bool) ([]ecv1beta1.Chart, []k0sv1beta1.Repository, error) {
	if !onlyDefaults {
		helmValues["embeddedClusterID"] = metrics.ClusterID().String()
		if a.airgapBundle != "" || a.isAirgap {
			helmValues["isAirgap"] = "true"
		} else {
			helmValues["isAirgap"] = "false"
		}
		helmValues["isHA"] = a.isHA
		// TODO(improveddr): remove this for testing
		extraEnv := []map[string]interface{}{{
			"name":  "ENABLE_IMPROVED_DR",
			"value": "true",
		}}
		if len(a.proxyEnv) > 0 {
			for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"} {
				extraEnv = append(extraEnv, map[string]interface{}{
					"name":  k,
					"value": a.proxyEnv[k],
				})
			}
		}
		helmValues["extraEnv"] = extraEnv

		var err error
		helmValues, err = helm.SetValue(helmValues, "kurlProxy.nodePort", runtimeconfig.AdminConsolePort())
		if err != nil {
			return nil, nil, fmt.Errorf("set helm values admin-console.kurlProxy.nodePort: %w", err)
		}
	}

	values, err := helm.MarshalValues(helmValues)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal helm values: %w", err)
	}

	chartName := Metadata.Location
	if AdminConsoleChartRepoOverride != "" {
		chartName = fmt.Sprintf("oci://proxy.replicated.com/anonymous/%s", AdminConsoleChartRepoOverride)
	}

	chartConfig := ecv1beta1.Chart{
		Name:         ReleaseName,
		ChartName:    chartName,
		Version:      Metadata.Version,
		Values:       string(values),
		TargetNS:     a.namespace,
		ForceUpgrade: ptr.To(false),
		Order:        5,
	}
	return []ecv1beta1.Chart{chartConfig}, nil, nil
}

func (a *AdminConsole) GetImages() []string {
	var images []string
	for _, image := range Metadata.Images {
		images = append(images, image.String())
	}
	return images
}

func (a *AdminConsole) GetAdditionalImages() []string {
	return nil
}

// Outro waits for the adminconsole to be ready.
func (a *AdminConsole) Outro(ctx context.Context, cli client.Client, k0sCfg *k0sv1beta1.ClusterConfig, releaseMetadata *types.ReleaseMetadata) error {
	loading := spinner.Start()
	loading.Infof("Waiting for the Admin Console to deploy")
	defer loading.Close()

	if err := createKotsPasswordSecret(ctx, cli, a.namespace, a.password); err != nil {
		return fmt.Errorf("unable to create kots password secret: %w", err)
	}

	if err := createKotsCAConfigmap(ctx, cli, a.namespace, a.privateCAs); err != nil {
		return fmt.Errorf("unable to create kots CA configmap: %w", err)
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
			AppSlug:          license.Spec.AppSlug,
			LicenseFile:      a.licenseFile,
			Namespace:        a.namespace,
			AirgapBundle:     a.airgapBundle,
			ConfigValuesFile: a.configValuesFile,
		}
		if err := kotscli.Install(installOpts, loading); err != nil {
			return err
		}
	}

	loading.Infof("Admin Console is ready!")

	return nil
}

// New creates a new AdminConsole object.
func New(
	namespace string,
	password string,
	licenseFile string,
	airgapBundle string,
	isAirgap bool,
	isHA bool,
	proxyEnv map[string]string,
	privateCAs map[string]string,
	configValuesFile string,
) (*AdminConsole, error) {
	return &AdminConsole{
		namespace:        namespace,
		password:         password,
		licenseFile:      licenseFile,
		airgapBundle:     airgapBundle,
		isAirgap:         isAirgap,
		isHA:             isHA,
		proxyEnv:         proxyEnv,
		privateCAs:       privateCAs,
		configValuesFile: configValuesFile,
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
func GetURL(networkInterface string, port int) string {
	ipaddr := runtimeconfig.TryDiscoverPublicIP()
	if ipaddr == "" {
		var err error
		ipaddr, err = netutils.FirstValidAddress(networkInterface)
		if err != nil {
			logrus.Errorf("unable to determine node IP address: %v", err)
			ipaddr = "NODE-IP-ADDRESS"
		}
	}
	return fmt.Sprintf("http://%s:%v", ipaddr, port)
}

func createRegistrySecret(ctx context.Context, cli client.Client, namespace string) error {
	if err := kubeutils.WaitForNamespace(ctx, cli, namespace, nil); err != nil {
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
	if err := kubeutils.WaitForNamespace(ctx, cli, namespace, nil); err != nil {
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

func createKotsCAConfigmap(ctx context.Context, cli client.Client, namespace string, cas map[string]string) error {
	kotsCAConfigmap := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kotsadm-private-cas",
			Namespace: namespace,
			Labels: map[string]string{
				"kots.io/kotsadm":                        "true",
				"replicated.com/disaster-recovery":       "infra",
				"replicated.com/disaster-recovery-chart": "admin-console",
			},
		},
		Data: cas,
	}

	err := cli.Create(ctx, &kotsCAConfigmap)
	if err != nil {
		return fmt.Errorf("unable to create kotsadm-private-cas configmap: %w", err)
	}

	return nil
}
