// Package adminconsole manages the Kots Admin Console helm chart installation
// or upgrade in the cluster.
package adminconsole

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/goods"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
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
)

// protectedFields are helm values that are not overwritten when upgrading the addon.
var protectedFields = []string{"automation", "embeddedClusterID", "isAirgap"}

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
	namespace    string
	useprompt    bool
	config       v1beta1.ClusterConfig
	licenseFile  string
	airgapBundle string
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

// GenerateHelmConfig generates the helm config for the adminconsole and writes the charts to
// the disk.
func (a *AdminConsole) GenerateHelmConfig(onlyDefaults bool) ([]v1beta1.Chart, []v1beta1.Repository, error) {
	if !onlyDefaults {
		if err := a.addPasswordToHelmValues(); err != nil {
			return nil, nil, fmt.Errorf("unable to add password to helm values: %w", err)
		}
		helmValues["embeddedClusterID"] = metrics.ClusterID().String()
		if a.airgapBundle != "" {
			helmValues["isAirgap"] = "true"
		} else {
			helmValues["isAirgap"] = "false"
		}
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

func (a *AdminConsole) GetAdditionalImages() []string {
	return nil
}

// MaskKotsOutputForOnline masks the kots cli output during online installations. For
// online installations we only want to print "Finalizing" until it is done and then
// print "Finished!".
func (a *AdminConsole) MaskKotsOutputForOnline() spinner.MaskFn {
	return func(message string) string {
		if strings.Contains(message, "Finished") {
			return message
		}
		return "Finalizing"
	}
}

// MaskKotsOutputForAirgap masks the kots cli output during airgap installations. This
// function replaces some of the messages being printed to the user so the output looks
// nicer.
func (a *AdminConsole) MaskKotsOutputForAirgap() spinner.MaskFn {
	current := "Uploading air gap bundle"
	return func(message string) string {
		switch {
		case strings.Contains(message, "Pushing application images"):
			current = message
		case strings.Contains(message, "Pushing embedded cluster artifacts"):
			current = message
		case strings.Contains(message, "Waiting for Admin Console"):
			current = "Finalizing"
		case strings.Contains(message, "Finished!"):
			current = message
		}
		return current
	}
}

// KostsOutputLineBreaker creates a line break (new spinner) when some of the messages
// are printed to the user. For example: after finishing all image uploads we want to
// have a new spinner for the artifacts upload.
func (a *AdminConsole) KostsOutputLineBreaker() spinner.LineBreakerFn {
	// finished is an auxiliary function that evaluates if a message refers to a
	// step that has been finished. We determine that by inspected if the message
	// contains %d/%d and both integers are equal.
	finished := func(message string) bool {
		matches := CounterRegex.FindStringSubmatch(message)
		if len(matches) != 3 {
			return false
		}
		var counter int
		if _, err := fmt.Sscanf(matches[1], "%d", &counter); err != nil {
			return false
		}
		var total int
		if _, err := fmt.Sscanf(matches[2], "%d", &total); err != nil {
			return false
		}
		return counter == total
	}

	var previous string
	var seen = map[string]bool{}
	return func(current string) (bool, string) {
		defer func() {
			previous = current
		}()

		// if we have already seen this message we certainly have already assessed
		// if a break line as necessary or not, on this case we return false so we
		// do not keep breaking lines indefinitely.
		if _, ok := seen[current]; ok {
			return false, ""
		}
		seen[current] = true

		// if the previous message evaluated does not relate to an end of a process
		// we don't want to break the line. i.e. we only want to break the line when
		// the previous evaluated message contains %d/%d and both integers are equal.
		if !finished(previous) {
			return false, ""
		}

		// if we are printing a message about pushing the embedded cluster artifacts
		// it means that we have finished with the images and we want to break the line.
		if strings.Contains(current, "Pushing embedded cluster artifacts") {
			return true, "Application images are ready!"
		}

		// if we are printing a message about the finalization of the installation it
		// means that the embedded cluster artifacts are ready and we want to break the
		// line.
		if current == "Finalizing" {
			return true, "Embedded cluster artifacts are ready!"
		}
		return false, ""
	}
}

// Outro waits for the adminconsole to be ready.
func (a *AdminConsole) Outro(ctx context.Context, cli client.Client) error {
	loading := spinner.Start()
	backoff := wait.Backoff{Steps: 60, Duration: 5 * time.Second, Factor: 1.0, Jitter: 0.1}
	loading.Infof("Waiting for Admin Console to deploy: 0/2 ready")

	if a.airgapBundle != "" {
		err := createRegistrySecret(ctx, cli, a.namespace)
		if err != nil {
			loading.Close()
			return fmt.Errorf("error creating registry secret: %v", err)
		}
	}

	var lasterr error
	if err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
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
	if a.licenseFile == "" {
		return nil
	}

	kotsBinPath, err := goods.MaterializeInternalBinary("kubectl-kots")
	if err != nil {
		return fmt.Errorf("unable to materialize kubectl-kots binary: %w", err)
	}
	defer os.Remove(kotsBinPath)

	license, err := helpers.ParseLicense(a.licenseFile)
	if err != nil {
		loading.CloseWithError()
		return fmt.Errorf("unable to parse license: %w", err)
	}

	var appVersionLabel string
	var channelSlug string
	if channelRelease, err := release.GetChannelRelease(); err != nil {
		loading.CloseWithError()
		return fmt.Errorf("unable to get channel release: %w", err)
	} else if channelRelease != nil {
		appVersionLabel = channelRelease.VersionLabel
		channelSlug = channelRelease.ChannelSlug
	}

	upstreamURI := license.Spec.AppSlug
	if channelSlug != "" && channelSlug != "stable" {
		upstreamURI = fmt.Sprintf("%s/%s", upstreamURI, channelSlug)
	}

	var lbreakfn spinner.LineBreakerFn
	maskfn := a.MaskKotsOutputForOnline()
	installArgs := []string{
		"install",
		upstreamURI,
		"--license-file",
		a.licenseFile,
		"--namespace",
		a.namespace,
		"--app-version-label",
		appVersionLabel,
		"--exclude-admin-console",
	}
	if a.airgapBundle != "" {
		installArgs = append(installArgs, "--airgap-bundle", a.airgapBundle)
		maskfn = a.MaskKotsOutputForAirgap()
		lbreakfn = a.KostsOutputLineBreaker()
	}

	loading = spinner.Start(spinner.WithMask(maskfn), spinner.WithLineBreaker(lbreakfn))
	if err := helpers.RunCommandWithWriter(loading, kotsBinPath, installArgs...); err != nil {
		loading.CloseWithError()
		return fmt.Errorf("unable to install the application: %w", err)
	}

	loading.Closef("Finished!")
	a.printSuccessMessage(license.Spec.AppSlug)
	return nil
}

// printSuccessMessage prints the success message when the admin console is online.
func (a *AdminConsole) printSuccessMessage(appSlug string) {
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
	successMessage := fmt.Sprintf("Visit the admin console to configure and install %s: %shttp://%s:%v%s",
		appSlug, successColor, ipaddr, DEFAULT_ADMIN_CONSOLE_NODE_PORT, colorReset,
	)
	logrus.Info(successMessage)
}

// New creates a new AdminConsole object.
func New(ns string, useprompt bool, config v1beta1.ClusterConfig, licenseFile string, airgapBundle string) (*AdminConsole, error) {
	return &AdminConsole{
		namespace:    ns,
		useprompt:    useprompt,
		config:       config,
		licenseFile:  licenseFile,
		airgapBundle: airgapBundle,
	}, nil
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
				"kots.io/kotsadm": "true",
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
