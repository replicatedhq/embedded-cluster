// Package adminconsole manages the Kots Admin Console helm chart installation
// or upgrade in the cluster.
package adminconsole

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8syaml "sigs.k8s.io/yaml"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
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
)

// protectedFields are helm values that are not overwritten when upgrading the addon.
var protectedFields = []string{"automation", "embeddedClusterID", "kotsApplication"}

const DEFAULT_ADMIN_CONSOLE_NODE_PORT = 30000

var helmValues = map[string]interface{}{
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
	namespace string
	useprompt bool
	airgap    bool
	config    v1beta1.ClusterConfig
	license   *kotsv1beta1.License
}

func (a *AdminConsole) askPassword() (string, error) {
	defaultPass := "password"
	if !a.useprompt {
		logrus.Info("Admin Console password set to: password")
		return defaultPass, nil
	}
	maxTries := 3
	for i := 0; i < maxTries; i++ {
		promptA := prompts.New().Password("Enter a new Admin Console password:")
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

// addLicenseAndVersionToHelmValues adds the embedded license to the helm values.
func (a *AdminConsole) addLicenseAndVersionToHelmValues() error {
	if a.license == nil {
		return nil
	}
	raw, err := k8syaml.Marshal(a.license)
	if err != nil {
		return fmt.Errorf("unable to marshal license: %w", err)
	}
	var appVersion string
	if channelRelease, err := release.GetChannelRelease(); err != nil {
		return fmt.Errorf("unable to get channel release: %w", err)
	} else if channelRelease != nil {
		appVersion = channelRelease.VersionLabel
	}

	isAirgap := "false"
	if a.airgap {
		isAirgap = "true"
	}

	helmValues["automation"] = map[string]interface{}{
		"appVersionLabel": appVersion,
		"license": map[string]interface{}{
			"slug": a.license.Spec.AppSlug,
			"data": string(raw),
		},
		"airgap": isAirgap,
	}

	if a.airgap {
		helmValues["registry"] = map[string]interface{}{
			"enabled":  "true",
			"readOnly": "true",
			"endpoint": "embedded-cluster-registry:5000",
		}
	}

	return nil
}

// getPasswordFromConfig returns the adminconsole password from the provided chart config.
func getPasswordFromConfig(chart *v1beta1.Chart) (string, error) {
	if chart.Values == "" {
		return "", nil
	}
	values := dig.Mapping{}
	if err := yaml.Unmarshal([]byte(chart.Values), &values); err != nil {
		return "", fmt.Errorf("unable to unmarshal values: %w", err)
	}
	if password, ok := values["password"].(string); ok {
		return password, nil
	}
	return "", nil
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

// addPasswordToHelmValues adds the adminconsole password to the helm values.
func (a *AdminConsole) addPasswordToHelmValues() error {
	curconfig := a.GetCurrentChartConfig()
	if curconfig == nil {
		pass, err := a.askPassword()
		if err != nil {
			return fmt.Errorf("unable to ask password: %w", err)
		}
		helmValues["password"] = pass
		return nil
	}
	pass, err := getPasswordFromConfig(curconfig)
	if err != nil {
		return fmt.Errorf("unable to get password from current config: %w", err)
	}
	helmValues["password"] = pass
	return nil
}

// addKotsApplicationToHelmValues extracts the embed application struct found in this binary
// and adds it to the helm values.
func (a *AdminConsole) addKotsApplicationToHelmValues() error {
	app, err := release.GetApplication()
	if err != nil {
		return fmt.Errorf("unable to get application: %w", err)
	} else if app == nil {
		helmValues["kotsApplication"] = "default value"
		return nil
	}
	helmValues["kotsApplication"] = string(app)
	return nil
}

// GenerateHelmConfig generates the helm config for the adminconsole and writes the charts to
// the disk.
func (a *AdminConsole) GenerateHelmConfig(onlyDefaults bool) ([]v1beta1.Chart, []v1beta1.Repository, error) {
	if !onlyDefaults {
		if err := a.addPasswordToHelmValues(); err != nil {
			return nil, nil, fmt.Errorf("unable to add password to helm values: %w", err)
		}
		if err := a.addKotsApplicationToHelmValues(); err != nil {
			return nil, nil, fmt.Errorf("unable to add kots app to helm values: %w", err)
		}
		helmValues["embeddedClusterID"] = metrics.ClusterID().String()
	}
	if err := a.addLicenseAndVersionToHelmValues(); err != nil {
		return nil, nil, fmt.Errorf("unable to add license to helm values: %w", err)
	}
	values, err := yaml.Marshal(helmValues)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal helm values: %w", err)
	}
	chartConfig := v1beta1.Chart{
		Name:      releaseName,
		ChartName: fmt.Sprintf("%s/%s", ChartURL, ChartName),
		Version:   Version,
		Values:    string(values),
		TargetNS:  a.namespace,
		Order:     5,
	}
	return []v1beta1.Chart{chartConfig}, nil, nil
}

// Outro waits for the adminconsole to be ready.
func (a *AdminConsole) Outro(ctx context.Context, cli client.Client) error {
	loading := spinner.Start()
	backoff := wait.Backoff{Steps: 60, Duration: 5 * time.Second, Factor: 1.0, Jitter: 0.1}
	loading.Infof("Waiting for Admin Console to deploy: 0/2 ready")

	err := createRegistrySecret(ctx, cli, a.namespace)
	if err != nil {
		loading.Close()
		return fmt.Errorf("error creating registry secret: %v", err)
	}

	var lasterr error
	if err = wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		var count int
		for _, name := range []string{"kotsadm-rqlite", "kotsadm"} {
			ready, err := kubeutils.IsStatefulSetReady(ctx, cli, a.namespace, name)
			if err != nil {
				lasterr = fmt.Errorf("error checking status of %s: %v", name, err)
				return false, nil
			}
			if ready {
				count++
			}
		}
		loading.Infof("Waiting for Admin Console to deploy: %d/2 ready", count)
		return count == 2, nil
	}); err != nil {
		if lasterr == nil {
			lasterr = err
		}
		loading.Close()
		return fmt.Errorf("error waiting for admin console: %v", lasterr)
	}
	loading.Closef("Admin Console is ready!")
	a.printSuccessMessage()
	return nil
}

// printSuccessMessage prints the success message when the admin console is online.
func (a *AdminConsole) printSuccessMessage() {
	successColor := "\033[32m"
	colorReset := "\033[0m"
	ipaddr := defaults.TryDiscoverPublicIP()
	if ipaddr == "" {
		var err error
		ipaddr, err = defaults.PreferredNodeIPAddress()
		if err != nil {
			logrus.Errorf("unable to determine node IP address: %v", err)
			ipaddr = "NODE-IP-ADDRESS"
		}
	}
	successMessage := fmt.Sprintf("Admin Console accessible at: %shttp://%s:%v%s",
		successColor, ipaddr, DEFAULT_ADMIN_CONSOLE_NODE_PORT, colorReset,
	)
	logrus.Info(successMessage)
}

// New creates a new AdminConsole object.
func New(ns string, useprompt bool, config v1beta1.ClusterConfig, license *kotsv1beta1.License, airgap bool) (*AdminConsole, error) {
	return &AdminConsole{
		namespace: ns,
		useprompt: useprompt,
		config:    config,
		license:   license,
		airgap:    airgap,
	}, nil
}

func createRegistrySecret(ctx context.Context, cli client.Client, namespace string) error {
	if err := kubeutils.WaitForNamespace(ctx, cli, namespace); err != nil {
		return err
	}
	registryIP, err := registry.GetRegistryClusterIP(ctx, cli)
	if err != nil {
		return fmt.Errorf("unable to get registry cluster ip: %w", err)
	}

	authString := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("embedded-cluster:%s", registry.GetRegistryPassword())))

	authConfig := fmt.Sprintf(`{"auths":{"%s:5000":{auth: "%s"}}`, registryIP, authString)

	registryCreds := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-creds",
			Namespace: namespace,
			Labels: map[string]string{
				"kots.io/kotsadm": "true",
			},
		},
		StringData: map[string]string{
			".dockerconfigjson": authConfig,
		},
		Type: "Opaque",
	}
	err = cli.Create(ctx, &registryCreds)
	if err != nil {
		return fmt.Errorf("unable to create registry-auth secret: %w", err)
	}

	return nil
}
