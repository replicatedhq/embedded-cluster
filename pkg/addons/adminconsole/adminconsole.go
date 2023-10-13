// Package adminconsole manages the Kots Admin Console helm chart installation
// or upgrade in the cluster.
package adminconsole

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"

	"github.com/replicatedhq/helmvm/pkg/addons/adminconsole/charts"
	"github.com/replicatedhq/helmvm/pkg/defaults"
	"github.com/replicatedhq/helmvm/pkg/prompts"
)

const (
	releaseName = "adminconsole"
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

type AdminConsole struct {
	customization AdminConsoleCustomization
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

func (a *AdminConsole) Version() (map[string]string, error) {
	latest, err := a.Latest()
	if err != nil {
		return nil, fmt.Errorf("unable to get latest version: %w", err)
	}
	return map[string]string{"AdminConsole": "v" + latest}, nil

}

// HostPreflight returns the host preflight objects found inside the adminconsole
// or as part of the embedded kots release (customization).
func (a *AdminConsole) HostPreflights() (*v1beta2.HostPreflightSpec, error) {
	return a.customization.hostPreflights()
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
func (a *AdminConsole) GenerateHelmConfig() ([]v1beta1.Chart, error) {

	latest, err := a.Latest()
	if err != nil {
		return nil, fmt.Errorf("unable to get latest version: %w", err)
	}

	chartConfig := v1beta1.Chart{
		Name:      releaseName,
		ChartName: defaults.PathToHelmChart("adminconsole", latest),
		Version:   latest,
		Values:    "",
		TargetNS:  a.namespace,
	}

	if err := a.addLicenseToHelmValues(); err != nil {
		return nil, fmt.Errorf("unable to add license to helm values: %w", err)
	}

	currentConfig, err := a.GetCurrentConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to get current config: %w", err)
	}

	currentPassword, err := getPasswordFromConfig(currentConfig)
	if err != nil {
		pass, err := a.askPassword()
		if err != nil {
			return nil, fmt.Errorf("unable to ask for password: %w", err)
		}
		helmValues["password"] = pass
	} else if currentPassword != "" {
		helmValues["password"] = currentPassword
	} else {
		pass, err := a.askPassword()
		if err != nil {
			return nil, fmt.Errorf("unable to ask for password: %w", err)
		}
		helmValues["password"] = pass
	}

	valuesStringData, err := yaml.Marshal(helmValues)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal helm values: %w", err)
	}
	chartConfig.Values = string(valuesStringData)

	err = a.WriteChartFile(latest)
	if err != nil {
		logrus.Fatalf("Unable to write chart file to disk: %s", err)
	}

	return []v1beta1.Chart{chartConfig}, nil

}

func (a *AdminConsole) WriteChartFile(version string) error {

	chartfile := fmt.Sprintf("%s-%s.tgz", releaseName, version)

	src, err := charts.FS.Open(chartfile)
	if err != nil {
		return fmt.Errorf("unable to open helm chart archive: %w", err)
	}

	dstpath := defaults.PathToHelmChart(releaseName, version)

	sourceFileByte, err := io.ReadAll(src)
	if err != nil {
		return fmt.Errorf("unable to read helm chart archive: %w", err)
	}

	err = os.WriteFile(dstpath, sourceFileByte, 0644)
	if err != nil {
		return fmt.Errorf("unable to write helm chart archive: %w", err)
	}

	return nil
}

func (a *AdminConsole) GetChartFileName() (string, error) {
	latest, err := a.Latest()
	if err != nil {
		return "", fmt.Errorf("unable to get latest version: %w", err)
	}
	return fmt.Sprintf("adminconsole-%s.tgz", latest), nil
}

func (a *AdminConsole) Latest() (string, error) {
	logrus.Infof("Finding latest Admin Console addon version")
	files, err := charts.FS.ReadDir(".")
	if err != nil {
		return "", fmt.Errorf("unable to read charts directory: %w", err)
	}
	latest := ""
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".tgz") {
			continue
		}
		trimmed := strings.TrimSuffix(file.Name(), ".tgz")
		slices := strings.Split(trimmed, "-")
		if len(slices) != 2 {
			return "", fmt.Errorf("invalid file name found: %s", file.Name())
		}
		currentV := fmt.Sprintf("%s", slices[1])
		if latest == "" {
			latest = currentV
			continue
		}
		if semver.Compare(latest, currentV) < 0 {
			latest = currentV
		}
	}
	logrus.Infof("Latest Admin Console version found: %s", latest)
	return latest, nil
}

func New(ns string, useprompt bool, config v1beta1.ClusterConfig) (*AdminConsole, error) {
	return &AdminConsole{
		namespace:     ns,
		useprompt:     useprompt,
		customization: AdminConsoleCustomization{},
		config:        config,
	}, nil
}
