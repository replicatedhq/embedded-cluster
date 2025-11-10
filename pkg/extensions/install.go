package extensions

import (
	"context"
	"sort"

	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
)

type ExtensionsProgress struct {
	Current int
	Total   int
}

func Install(ctx context.Context, hcli helm.Client, progressChan chan<- ExtensionsProgress) error {
	if progressChan != nil {
		defer close(progressChan)
	}

	// check if there are any extensions
	if len(config.AdditionalCharts()) == 0 {
		return nil
	}

	if err := addRepos(ctx, hcli, config.AdditionalRepositories()); err != nil {
		return errors.Wrap(err, "add additional helm repositories")
	}

	// sort by order first
	sorted := config.AdditionalCharts()
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Order < sorted[j].Order
	})

	numExtensions := len(sorted)

	for i, ext := range sorted {
		if progressChan != nil {
			progressChan <- ExtensionsProgress{Current: i + 1, Total: numExtensions}
		}

		if err := install(ctx, hcli, ext); err != nil {
			return errors.Wrapf(err, "install extension %s", ext.Name)
		}
	}

	return nil
}
