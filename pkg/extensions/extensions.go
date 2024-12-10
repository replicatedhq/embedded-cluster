package extensions

import (
	"context"
	"sort"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	helmrepo "helm.sh/helm/v3/pkg/repo"
)

func Install(ctx context.Context) error {
	// check if there are any extensions
	if len(config.AdditionalCharts()) == 0 {
		return nil
	}

	loading := spinner.Start()
	defer loading.Close()

	hcli, err := helm.NewHelm(helm.HelmOptions{
		K0sVersion: versions.K0sVersion,
	})
	if err != nil {
		return errors.Wrap(err, "create helm client")
	}

	for _, k0sRepo := range config.AdditionalRepositories() {
		logrus.Debugf("Adding helm repository %s", k0sRepo.Name)

		helmRepo := &helmrepo.Entry{
			Name:     k0sRepo.Name,
			URL:      k0sRepo.URL,
			Username: k0sRepo.Username,
			Password: k0sRepo.Password,
			CertFile: k0sRepo.CertFile,
			KeyFile:  k0sRepo.KeyFile,
			CAFile:   k0sRepo.CAFile,
		}
		if k0sRepo.Insecure != nil {
			helmRepo.InsecureSkipTLSverify = *k0sRepo.Insecure
		}
		if err := hcli.AddRepo(helmRepo); err != nil {
			return errors.Wrapf(err, "add helm repository %s", k0sRepo.Name)
		}
	}

	// sort by order first
	sorted := config.AdditionalCharts()
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Order < sorted[j].Order
	})

	for _, ext := range sorted {
		loading.Infof("Installing %s", ext.Name)

		var values map[string]interface{}
		if err := yaml.Unmarshal([]byte(ext.Values), &values); err != nil {
			return errors.Wrap(err, "unmarshal values")
		}

		_, err = hcli.Install(ctx, helm.InstallOptions{
			ReleaseName:  ext.Name,
			ChartPath:    ext.ChartName,
			ChartVersion: ext.Version,
			Values:       values,
			Namespace:    ext.TargetNS,
			Timeout:      ext.Timeout.Duration,
		})
		if err != nil {
			return errors.Wrap(err, "helm install")
		}
	}

	return nil
}
