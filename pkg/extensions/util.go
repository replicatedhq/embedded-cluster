package extensions

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/sirupsen/logrus"
	helmrepo "helm.sh/helm/v3/pkg/repo"
)

func addRepos(ctx context.Context, hcli helm.Client, repos []k0sv1beta1.Repository) error {
	for _, r := range repos {
		logrus.Debugf("Adding helm repository %s", r.Name)

		helmRepo := &helmrepo.Entry{
			Name:     r.Name,
			URL:      r.URL,
			Username: r.Username,
			Password: r.Password,
			CertFile: r.CertFile,
			KeyFile:  r.KeyFile,
			CAFile:   r.CAFile,
		}
		if r.Insecure != nil {
			helmRepo.InsecureSkipTLSverify = *r.Insecure
		}
		if err := hcli.AddRepo(ctx, helmRepo); err != nil {
			return errors.Wrapf(err, "add helm repository %s", r.Name)
		}
	}

	return nil
}

func install(ctx context.Context, hcli helm.Client, ext ecv1beta1.Chart) error {
	values, err := helm.UnmarshalValues(ext.Values)
	if err != nil {
		return errors.Wrap(err, "unmarshal values")
	}

	_, err = hcli.Install(ctx, helm.InstallOptions{
		ReleaseName:  ext.Name,
		ChartPath:    ext.ChartName,
		ChartVersion: ext.Version,
		Values:       values,
		Namespace:    ext.TargetNS,
		Timeout:      ext.Timeout.Duration,
		// TODO: Do we need to set LogFn?
	})
	if err != nil {
		return errors.Wrap(err, "helm install")
	}

	return nil
}

func upgrade(ctx context.Context, hcli helm.Client, ext ecv1beta1.Chart) error {
	values, err := helm.UnmarshalValues(ext.Values)
	if err != nil {
		return errors.Wrap(err, "unmarshal values")
	}

	opts := helm.UpgradeOptions{
		ReleaseName:  ext.Name,
		ChartPath:    ext.ChartName,
		ChartVersion: ext.Version,
		Values:       values,
		Namespace:    ext.TargetNS,
		Timeout:      ext.Timeout.Duration,
		Force:        true, // this was the default in k0s
		// TODO: Do we need to set LogFn?
	}
	if ext.ForceUpgrade != nil {
		opts.Force = *ext.ForceUpgrade
	}
	_, err = hcli.Upgrade(ctx, opts)
	if err != nil {
		return errors.Wrap(err, "helm upgrade")
	}

	return nil
}

func uninstall(ctx context.Context, hcli helm.Client, ext ecv1beta1.Chart) error {
	err := hcli.Uninstall(ctx, helm.UninstallOptions{
		ReleaseName: ext.Name,
		Namespace:   ext.TargetNS,
		Wait:        true,
	})
	if err != nil {
		return errors.Wrap(err, "helm uninstall")
	}

	return nil
}

type diffResult struct {
	Action helmAction
	Ext    ecv1beta1.Chart
}

func diffExtensions(oldExts, newExts ecv1beta1.Extensions) []diffResult {
	oldCharts := make(map[string]ecv1beta1.Chart)
	newCharts := make(map[string]ecv1beta1.Chart)

	if oldExts.Helm != nil {
		for _, chart := range oldExts.Helm.Charts {
			oldCharts[chart.Name] = chart
		}
	}
	if newExts.Helm != nil {
		for _, chart := range newExts.Helm.Charts {
			newCharts[chart.Name] = chart
		}
	}

	var results []diffResult

	for name, newChart := range newCharts {
		r := diffResult{
			Ext: newChart,
		}
		oldChart, ok := oldCharts[name]
		if !ok {
			// chart was added.
			r.Action = actionInstall
		} else if !reflect.DeepEqual(oldChart, newChart) {
			r.Action = actionUpgrade
		} else {
			r.Action = actionNoChange
		}
		results = append(results, r)
	}

	for name, oldChart := range oldCharts {
		_, ok := newCharts[name]
		if !ok {
			// chart was removed.
			results = append(results, diffResult{
				Action: actionUninstall,
				Ext:    oldChart,
			})
		}
	}

	// sort next extensions by order
	sort.Slice(results, func(i, j int) bool {
		return results[i].Ext.Order < results[j].Ext.Order
	})

	return results
}

func conditionName(ext ecv1beta1.Chart) string {
	return fmt.Sprintf("%s-%s", ext.TargetNS, ext.Name)
}
