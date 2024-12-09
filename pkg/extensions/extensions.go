package extensions

import (
	"context"

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
	loading := spinner.Start()
	defer loading.Close()

	helm, err := helm.NewHelm(helm.HelmOptions{
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
		if err := helm.AddRepo(helmRepo); err != nil {
			return errors.Wrapf(err, "add helm repository %s", k0sRepo.Name)
		}
	}

	for _, ext := range config.AdditionalCharts() {
		loading.Infof("Installing %s", ext.Name)

		var values map[string]interface{}
		if err := yaml.Unmarshal([]byte(ext.Values), &values); err != nil {
			return errors.Wrap(err, "unmarshal values")
		}

		_, err = helm.Install(ctx, ext.Name, ext.ChartName, ext.Version, values, ext.TargetNS)
		if err != nil {
			return errors.Wrap(err, "helm install")
		}
	}

	return nil
}
