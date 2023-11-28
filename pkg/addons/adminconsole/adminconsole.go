// Package adminconsole manages the Kots Admin Console helm chart installation
// or upgrade in the cluster.
package adminconsole

import (
	"context"
	"fmt"
	"time"

	"github.com/k0sproject/dig"
	helmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8syaml "sigs.k8s.io/yaml"

	"github.com/replicatedhq/embedded-cluster/pkg/customization"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	pb "github.com/replicatedhq/embedded-cluster/pkg/progressbar"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
)

const (
	releaseName = "admin-console"
)

// Overwritten by -ldflags in Makefile
var (
	ChartURL  = "https://url"
	ChartName = "name"
	Version   = "v0.0.0"
)

var helmValues = map[string]interface{}{
	"password":      "password",
	"minimalRBAC":   false,
	"isHelmManaged": false,
	"service": map[string]interface{}{
		"type":     "NodePort",
		"nodePort": 30000,
	},
}

// AdminConsole manages the admin console helm chart installation.
type AdminConsole struct {
	customization customization.AdminConsole
	namespace     string
	useprompt     bool
	config        v1beta1.ClusterConfig
}

func (a *AdminConsole) askPassword() (string, error) {
	defaultPass := "password"
	if !a.useprompt {
		fmt.Println("Admin Console password set to: password")
		return defaultPass, nil
	}
	maxTries := 3
	for i := 0; i < maxTries; i++ {
		promptA := prompts.New().Password("Enter a new Admin Console password:")
		promptB := prompts.New().Password("Confirm password:")

		if promptA == promptB {
			return promptA, nil
		}
		fmt.Println("Passwords don't match, please try again.")
	}
	return "", fmt.Errorf("Unable to set Admin Console password after %d tries", maxTries)
}

// Version returns the embedded admin console version.
func (a *AdminConsole) Version() (map[string]string, error) {
	return map[string]string{"AdminConsole": "v" + Version}, nil
}

// HostPreflights returns the host preflight objects found inside the adminconsole
// or as part of the embedded kots release (customization).
func (a *AdminConsole) HostPreflights() (*v1beta2.HostPreflightSpec, error) {
	return a.customization.HostPreflights()
}

// addLicenseToHelmValues adds the embedded license to the helm values.
func (a *AdminConsole) addLicenseToHelmValues() error {
	license, err := a.customization.License()
	if err != nil {
		return fmt.Errorf("unable to get license: %w", err)
	}
	if license == nil {
		return nil
	}
	raw, err := k8syaml.Marshal(license)
	if err != nil {
		return fmt.Errorf("unable to marshal license: %w", err)
	}
	helmValues["automation"] = map[string]interface{}{
		"license": map[string]interface{}{
			"slug": license.Spec.AppSlug,
			"data": string(raw),
		},
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
	app, err := customization.AdminConsole{}.Application()
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
func (a *AdminConsole) GenerateHelmConfig() ([]helmv1beta1.Chart, []v1beta1.Repository, error) {
	if err := a.addPasswordToHelmValues(); err != nil {
		return nil, nil, fmt.Errorf("unable to add password to helm values: %w", err)
	}
	if err := a.addKotsApplicationToHelmValues(); err != nil {
		return nil, nil, fmt.Errorf("unable to add kots app to helm values: %w", err)
	}
	if err := a.addLicenseToHelmValues(); err != nil {
		return nil, nil, fmt.Errorf("unable to add license to helm values: %w", err)
	}
	values, err := yaml.Marshal(helmValues)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal helm values: %w", err)
	}
	chartConfig := helmv1beta1.Chart{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Chart",
			APIVersion: "helm.k0sproject.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      releaseName,
			Namespace: "kube-system",
		},
		Spec: helmv1beta1.ChartSpec{
			ReleaseName: releaseName,
			ChartName:   fmt.Sprintf("%s/%s", ChartURL, ChartName),
			Version:     Version,
			Values:      string(values),
			Namespace:   a.namespace,
			Order:       4,
		},
		Status: helmv1beta1.ChartStatus{},
	}
	return []helmv1beta1.Chart{chartConfig}, nil, nil
}

// Outro waits for the adminconsole to be ready.
func (a *AdminConsole) Outro(ctx context.Context, cli client.Client) error {
	loading := pb.Start()
	backoff := wait.Backoff{Steps: 60, Duration: 5 * time.Second, Factor: 1.0, Jitter: 0.1}
	loading.Infof("Waiting for Admin Console to deploy: 0/3 ready")
	var lasterr error
	if err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		var count int
		ready, err := kubeutils.IsDeploymentReady(ctx, cli, a.namespace, "kotsadm")
		if err != nil {
			lasterr = fmt.Errorf("error checking status of kotsadm: %v", err)
			return false, nil
		}
		if ready {
			count++
		}
		for _, name := range []string{"kotsadm-rqlite", "kotsadm-minio"} {
			ready, err := kubeutils.IsStatefulSetReady(ctx, cli, a.namespace, name)
			if err != nil {
				lasterr = fmt.Errorf("error checking status of %s: %v", name, err)
				return false, nil
			}
			if ready {
				count++
			}
		}
		loading.Infof("Waiting for Admin Console to deploy: %d/3 ready", count)
		return count == 3, nil
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
			fmt.Println(fmt.Errorf("unable to determine node IP address: %w", err))
			ipaddr = "NODE-IP-ADDRESS"
		}
	}
	nodePort := helmValues["service"].(map[string]interface{})["nodePort"]
	successMessage := fmt.Sprintf("Admin Console accessible at: %shttp://%s:%v%s", successColor, ipaddr, nodePort, colorReset)
	fmt.Println(successMessage)
}

// New creates a new AdminConsole object.
func New(ns string, useprompt bool, config v1beta1.ClusterConfig) (*AdminConsole, error) {
	return &AdminConsole{
		namespace:     ns,
		useprompt:     useprompt,
		customization: customization.AdminConsole{},
		config:        config,
	}, nil
}
