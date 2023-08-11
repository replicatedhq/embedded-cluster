// Package addons manages the default addons installations in the cluster. Addons are
// mostly Helm Charts, but can also be other resources as the project evolves. All of
// the AddOns must implement the AddOn interface.
package addons

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/replicatedhq/helmvm/pkg/addons/adminconsole"
	"github.com/replicatedhq/helmvm/pkg/addons/custom"
	"github.com/replicatedhq/helmvm/pkg/addons/openebs"
	"github.com/replicatedhq/helmvm/pkg/progressbar"
)

type Applier struct {
	kubeclient client.Client
	addons     map[string]AddOn
}

// AddOn is the interface that all addons must implement.
type AddOn interface {
	Apply(ctx context.Context) error
}

// Apply applies all registered addons to the cluster. Simply calls Apply on
// each addon.
func (a *Applier) Apply(ctx context.Context) error {
	if err := a.waitForKubernetes(ctx); err != nil {
		return fmt.Errorf("unable to wait for kubernetes: %w", err)
	}
	for name, addon := range a.addons {
		logrus.Infof("Apply addon %s.", name)
		if err := addon.Apply(ctx); err != nil {
			return err
		}
		logrus.Infof("Addon %s applied.", name)
	}
	return nil
}

// waitForKubernetes waits until we manage to make a successful connection to the
// Kubernetes API server.
func (a *Applier) waitForKubernetes(ctx context.Context) error {
	pb, end := progressbar.Start()
	defer func() {
		pb.Infof("Kubernetes API server is ready.")
		pb.Close()
		<-end
	}()
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	counter := 1
	pb.Infof("1/n Waiting for Kubernetes API server to be ready.")
	for {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return ctx.Err()
		}
		counter++
		if err := a.kubeclient.List(ctx, &corev1.NamespaceList{}); err != nil {
			pb.Infof("%d/n Waiting for Kubernetes API server to be ready.", counter)
			continue
		}
		return nil
	}
}

// NewApplier creates a new Applier instance with all addons registered.
func NewApplier(prompt bool) (*Applier, error) {
	k8slogger := zap.New(func(o *zap.Options) {
		o.DestWriter = io.Discard
	})
	log.SetLogger(k8slogger)
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to process kubernetes config: %w", err)
	}
	kubecli, err := client.New(cfg, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("unable to create kubernetes client: %w", err)
	}
	applier := &Applier{
		addons:     map[string]AddOn{},
		kubeclient: kubecli,
	}
	logger := logrus.WithField("addon", "openebs")
	obs, err := openebs.New("helmvm", logger.Infof)
	if err != nil {
		return nil, fmt.Errorf("unable to create admin console addon: %w", err)
	}
	applier.addons["openebs"] = obs
	logger = logrus.WithField("addon", "adminconsole")
	aconsole, err := adminconsole.New("helmvm", prompt, kubecli, logger.Infof)
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
