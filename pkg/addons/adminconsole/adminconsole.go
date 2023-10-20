// Package adminconsole manages the Kots Admin Console helm chart installation
// or upgrade in the cluster.
package adminconsole

import (
	"context"
	"fmt"
	"time"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/helmvm/pkg/customization"
	"github.com/replicatedhq/helmvm/pkg/defaults"
	pb "github.com/replicatedhq/helmvm/pkg/progressbar"
	"github.com/replicatedhq/helmvm/pkg/prompts"
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
	raw, err := yaml.Marshal(license)
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

// GetPasswordFromConfig returns the adminconsole password from the provided chart config.
func getPasswordFromConfig(chart v1beta1.Chart) (string, error) {
	values := dig.Mapping{}
	if chart.Values == "" {
		return "", fmt.Errorf("unable to find adminconsole chart values in cluster config")
	}
	err := yaml.Unmarshal([]byte(chart.Values), &values)
	if err != nil {
		return "", fmt.Errorf("unable to unmarshal values: %w", err)
	}
	if password, ok := values["password"].(string); ok {
		return password, nil
	}
	return "", fmt.Errorf("unable to find password in cluster config")
}

// GetCurrentConfig returns the current adminconsole chart config from the cluster config.
func (a *AdminConsole) GetCurrentConfig() (v1beta1.Chart, error) {
	nilChart := v1beta1.Chart{}
	if a.config.Spec == nil {
		return nilChart, fmt.Errorf("unable to find spec in cluster config")
	}
	spec := a.config.Spec
	if spec.Extensions == nil {
		return nilChart, fmt.Errorf("unable to find extensions in cluster config")
	}
	extensions := spec.Extensions
	if extensions.Helm == nil {
		return nilChart, fmt.Errorf("unable to find helm extensions in cluster config")
	}
	chartList := a.config.Spec.Extensions.Helm.Charts
	for _, chart := range chartList {
		if chart.Name == "adminconsole" {
			return chart, nil
		}
	}
	return nilChart, fmt.Errorf("unable to find adminconsole chart in cluster config")
}

// GenerateHelmConfig generates the helm config for the adminconsole
// and writes the charts to the disk.
func (a *AdminConsole) GenerateHelmConfig() ([]v1beta1.Chart, []v1beta1.Repository, error) {
	chartConfig := v1beta1.Chart{
		Name:      releaseName,
		ChartName: fmt.Sprintf("%s/%s", ChartURL, ChartName),
		Version:   Version,
		Values:    "",
		TargetNS:  a.namespace,
	}
	if err := a.addLicenseToHelmValues(); err != nil {
		return nil, nil, fmt.Errorf("unable to add license to helm values: %w", err)
	}
	currentConfig, err := a.GetCurrentConfig()
	if err == nil {
		currentPassword, err := getPasswordFromConfig(currentConfig)
		if err != nil {
			pass, err := a.askPassword()
			if err != nil {
				return nil, nil, fmt.Errorf("unable to ask for password: %w", err)
			}
			helmValues["password"] = pass
		} else if currentPassword != "" {
			helmValues["password"] = currentPassword
		}
	} else {
		pass, err := a.askPassword()
		if err != nil {
			return nil, nil, fmt.Errorf("unable to ask for password: %w", err)
		}
		helmValues["password"] = pass
	}
	cust, err := customization.AdminConsole{}.ExtractCustomization()
	if err == nil {
		if cust != nil && cust.Application != nil {
			helmValues["kotsApplication"] = string(cust.Application)
		} else {
			helmValues["kotsApplication"] = "default value"
		}
	} else {
		helmValues["kotsApplication"] = "error value"
	}
	valuesStringData, err := yaml.Marshal(helmValues)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal helm values: %w", err)
	}
	chartConfig.Values = string(valuesStringData)
	return []v1beta1.Chart{chartConfig}, nil, nil
}

// Outro waits for the adminconsole to be ready.
func (a *AdminConsole) Outro(ctx context.Context, cli client.Client) error {
	progressBar := pb.Start()
	backoff := wait.Backoff{Steps: 60, Duration: 5 * time.Second, Factor: 1.0, Jitter: 0.1}
	duration := a.totalTimeoutDuration(backoff)
	fmt.Printf("Waiting for Admin Console to be ready. This may take up to %v\n", duration)
	progressBar.Infof("Waiting for Admin Console to be ready: 0/3 resources ready")
	var lasterr error
	if err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		var count int
		ready, err := a.isDeploymentReady(ctx, cli)
		if err != nil {
			lasterr = fmt.Errorf("error checking status of kotsadm: %v", err)
			return false, nil
		}
		if ready {
			count++
		}
		for _, name := range []string{"kotsadm-rqlite", "kotsadm-minio"} {
			ready, err := a.isStatefulSetReady(ctx, cli, name)
			if err != nil {
				lasterr = fmt.Errorf("error checking status of %s: %v", name, err)
				return false, nil
			}
			if ready {
				count++
			}
		}
		progressBar.Infof("Waiting for Admin Console to be ready: %d/3 resources ready", count)
		return count == 3, nil
	}); err != nil {
		progressBar.Close()
		return fmt.Errorf("timed out waiting for admin console: %v", lasterr)
	}
	progressBar.Close()
	a.printSuccessMessage()
	return nil
}

// totalTimeoutDuration calculates the total time represented by the given backoff.
func (a *AdminConsole) totalTimeoutDuration(backoff wait.Backoff) time.Duration {
	var total time.Duration
	duration := backoff.Duration
	for i := 0; i < backoff.Steps; i++ {
		total += duration
		duration = time.Duration(float64(duration) * backoff.Factor)
	}
	return total
}

// isDeploymentReady checks if the admin console deployment is ready.
func (a *AdminConsole) isDeploymentReady(ctx context.Context, cli client.Client) (bool, error) {
	var deploy appsv1.Deployment
	nsn := types.NamespacedName{Namespace: a.namespace, Name: "kotsadm"}
	if err := cli.Get(ctx, nsn, &deploy); err != nil {
		return false, err
	}
	if deploy.Spec.Replicas == nil {
		return false, nil
	}
	return deploy.Status.ReadyReplicas == *deploy.Spec.Replicas, nil
}

// isStatefulSetReady checks if the provided statefulset is ready.
func (a *AdminConsole) isStatefulSetReady(ctx context.Context, cli client.Client, name string) (bool, error) {
	var statefulset appsv1.StatefulSet
	nsn := types.NamespacedName{Namespace: a.namespace, Name: name}
	if err := cli.Get(ctx, nsn, &statefulset); err != nil {
		return false, err
	}
	if statefulset.Spec.Replicas == nil {
		return false, nil
	}
	return statefulset.Status.ReadyReplicas == *statefulset.Spec.Replicas, nil
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
	successMessage := fmt.Sprintf("Admin Console accessible at: %shttps://%s:%v%s", successColor, ipaddr, nodePort, colorReset)
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
