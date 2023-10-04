// Package adminconsole manages the Kots Admin Console helm chart installation
// or upgrade in the cluster.
package adminconsole

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/k0sproject/dig"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"

	"github.com/replicatedhq/helmvm/pkg/addons/adminconsole/charts"
	"github.com/replicatedhq/helmvm/pkg/defaults"
	"github.com/replicatedhq/helmvm/pkg/prompts"
)

const (
	releaseName = "adminconsole"
	appVersion  = "1.100.1"
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
}

func (a *AdminConsole) askPassword() (string, error) {
	if !a.useprompt {
		fmt.Println("Admin Console password set to: password")
		return "password", nil
	}
	return prompts.New().Password("Enter a new Admin Console password:"), nil
}

func (a *AdminConsole) Version() (map[string]string, error) {
	latest, err := a.Latest()
	if err != nil {
		return nil, fmt.Errorf("unable to get latest version: %w", err)
	}
	return map[string]string{"AdminConsole": latest}, nil

}

// HostPreflight returns the host preflight objects found inside the adminconsole
// or as part of the embedded kots release (customization).
func (a *AdminConsole) HostPreflights() (*v1beta2.HostPreflightSpec, error) {
	return a.customization.hostPreflights()
}

// addLicenseToHelmValues adds the embedded license to the helm values.
func (a *AdminConsole) addLicenseToHelmValues() error {
	license, err := a.customization.license()
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

// GenerateHelmConfig generates the helm config for the adminconsole
// and writes the charts to the disk.
func (a *AdminConsole) GenerateHelmConfig(ctx *cli.Context) ([]dig.Mapping, error) {

	chartConfig := dig.Mapping{
		"name":      releaseName,
		"namespace": a.namespace,
		"version":   appVersion,
	}

	chartConfigs := []dig.Mapping{}

	chartConfig["chartName"] = filepath.Join(defaults.HelmChartSubDir(), a.GetChartFileName())

	if err := a.addLicenseToHelmValues(); err != nil {
		return chartConfigs, fmt.Errorf("unable to add license to helm values: %w", err)
	}

	pass, err := a.askPassword()
	if err != nil {
		return chartConfigs, fmt.Errorf("unable to ask for password: %w", err)
	}

	helmValues["password"] = pass

	valuesStringData, err := yaml.Marshal(helmValues)
	if err != nil {
		return chartConfigs, err
	}
	chartConfig["values"] = string(valuesStringData)

	chartConfigs = append(chartConfigs, chartConfig)

	err = a.WriteChartFile()
	if err != nil {
		logrus.Fatalf("could not write chart file: %s", err)
	}

	return chartConfigs, nil

}

func (a *AdminConsole) WriteChartFile() error {
	chartfile := a.GetChartFileName()
	src, err := charts.FS.Open(chartfile)
	if err != nil {
		return fmt.Errorf("could not load chart file: %w", err)
	}

	dstpath := filepath.Join(defaults.HelmChartSubDir(), chartfile)
	dst, err := os.Create(dstpath)
	defer func() {
		if err := dst.Close(); err != nil {
			logrus.Errorf("error closing file %s: %s", dstpath, err)
		}
	}()
	if err != nil {
		return fmt.Errorf("could not write helm chart archive: %w", err)
	}

	_, err = io.Copy(dst, src)

	return err
}

func (a *AdminConsole) GetChartFileName() string {
	return fmt.Sprintf("adminconsole-%s.tgz", appVersion)
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
		currentV := fmt.Sprintf("v%s", slices[1])
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

func New(ns string, useprompt bool) (*AdminConsole, error) {
	return &AdminConsole{
		namespace:     ns,
		useprompt:     useprompt,
		customization: AdminConsoleCustomization{},
	}, nil
}
