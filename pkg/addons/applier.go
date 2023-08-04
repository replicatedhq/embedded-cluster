// Package addons manages the default addons installations in the cluster. Addons are
// mostly Helm Charts, but can also be other resources as the project evolves. All of
// the AddOns must implement the AddOn interface.
package addons

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/replicatedhq/helmvm/pkg/addons/adminconsole"
	"github.com/replicatedhq/helmvm/pkg/addons/custom"
	"github.com/replicatedhq/helmvm/pkg/addons/openebs"
)

type Applier struct {
	addons map[string]AddOn
}

// AddOn is the interface that all addons must implement.
type AddOn interface {
	Apply(ctx context.Context) error
}

// Apply applies all registered addons to the cluster. Simply calls Apply on
// each addon.
func (a *Applier) Apply(ctx context.Context) error {
	for name, addon := range a.addons {
		logrus.Infof("Apply addon %s.", name)
		if err := addon.Apply(ctx); err != nil {
			return err
		}
		logrus.Infof("Addon %s applied.", name)
	}
	return nil
}

// NewApplier creates a new Applier instance with all addons registered.
func NewApplier() (*Applier, error) {
	applier := &Applier{map[string]AddOn{}}
	logger := logrus.WithField("addon", "openebs")
	obs, err := openebs.New("helmvm", logger.Infof)
	if err != nil {
		return nil, fmt.Errorf("unable to create admin console addon: %w", err)
	}
	applier.addons["openebs"] = obs
	logger = logrus.WithField("addon", "adminconsole")
	aconsole, err := adminconsole.New("helmvm", logger.Infof)
	if err != nil {
		return nil, fmt.Errorf("unable to create admin console addon: %w", err)
	}
	applier.addons["adminconsole"] = aconsole
	logger = logrus.WithField("addon", "custom")
	custom, err := custom.New("helmvm", logger.Infof)
	if err != nil {
		return nil, fmt.Errorf("unable to create admin console addon: %w", err)
	}
	applier.addons["custom"] = custom
	return applier, nil
}
