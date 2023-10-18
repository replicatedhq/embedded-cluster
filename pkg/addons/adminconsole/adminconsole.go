// Package adminconsole manages the Kots Admin Console helm chart installation
// or upgrade in the cluster.
package adminconsole

import (
	"fmt"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v3"
	yamlFoo "sigs.k8s.io/yaml"

	"github.com/replicatedhq/helmvm/pkg/customization"
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

	result, err := yamlFoo.Marshal(chartConfig)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(result))

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

// New creates a new AdminConsole object.
func New(ns string, useprompt bool, config v1beta1.ClusterConfig) (*AdminConsole, error) {
	return &AdminConsole{
		namespace:     ns,
		useprompt:     useprompt,
		customization: customization.AdminConsole{},
		config:        config,
	}, nil
}
