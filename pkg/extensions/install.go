package extensions

import (
	"context"
	"sort"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

func Install(ctx context.Context, hcli helm.Client) error {
	// check if there are any extensions
	if len(config.AdditionalCharts()) == 0 {
		return nil
	}

	loading := spinner.Start()
	loading.Infof("Installing additional components")

	if err := addRepos(hcli, config.AdditionalRepositories()); err != nil {
		loading.ErrorClosef("Failed to install additional components")
		return errors.Wrap(err, "add additional helm repositories")
	}

	// sort by order first
	sorted := config.AdditionalCharts()
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Order < sorted[j].Order
	})

	numExtensions := len(sorted)

	for i, ext := range sorted {
		loading.Infof("Installing additional components (%d/%d)", i+1, numExtensions)

		if err := install(ctx, hcli, ext); err != nil {
			loading.ErrorClosef("Failed to install additional components")
			return errors.Wrapf(err, "install extension %s", ext.Name)
		}
	}

	loading.Closef("Additional components are ready")

	return nil
}
