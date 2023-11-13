// Package addons manages the default addons installations in the cluster. Addons are
// mostly Helm Charts, but can also be other resources as the project evolves. All of
// the AddOns must implement the AddOn interface.
package addons

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/custom"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	pb "github.com/replicatedhq/embedded-cluster/pkg/progressbar"
)

// AddOn is the interface that all addons must implement.
type AddOn interface {
	Version() (map[string]string, error)
	HostPreflights() (*v1beta2.HostPreflightSpec, error)
	GenerateHelmConfig() ([]v1beta1.Chart, []v1beta1.Repository, error)
	Outro(context.Context, client.Client) error
}

// Applier is an entity that applies (installs and updates) addons in the cluster.
type Applier struct {
	disabledAddons map[string]bool
	prompt         bool
	verbose        bool
	config         v1beta1.ClusterConfig
}

// Outro runs the outro in all enabled add-ons.
func (a *Applier) Outro(ctx context.Context) error {
	kcli, err := a.kubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}
	addons, err := a.load()
	if err != nil {
		return fmt.Errorf("unable to load addons: %w", err)
	}
	for _, addon := range addons {
		if err := addon.Outro(ctx, kcli); err != nil {
			return err
		}
	}
	return nil
}

// GenerateHelmConfigs generates the helm config for all the embedded charts.
func (a *Applier) GenerateHelmConfigs() ([]v1beta1.Chart, []v1beta1.Repository, error) {
	charts := []v1beta1.Chart{}
	repositories := []v1beta1.Repository{}
	addons, err := a.load()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to load addons: %w", err)
	}
	for _, addon := range addons {
		addonChartConfig, addonRepositoryConfig, err := addon.GenerateHelmConfig()
		if err != nil {
			return nil, nil, fmt.Errorf("unable to generate helm config for %s: %w", addon, err)
		}
		charts = append(charts, addonChartConfig...)
		repositories = append(repositories, addonRepositoryConfig...)
	}
	return charts, repositories, nil
}

// HostPreflights reads all embedded host preflights from all add-ons and returns them
// merged in a single HostPreflightSpec.
func (a *Applier) HostPreflights() (*v1beta2.HostPreflightSpec, error) {
	addons, err := a.load()
	if err != nil {
		return nil, fmt.Errorf("unable to load addons: %w", err)
	}
	allpf := &v1beta2.HostPreflightSpec{}
	for _, addon := range addons {
		hpf, err := addon.HostPreflights()
		if err != nil {
			return nil, fmt.Errorf("unable to get preflights for %s: %w", addon, err)
		} else if hpf == nil {
			continue
		}
		allpf.Collectors = append(allpf.Collectors, hpf.Collectors...)
		allpf.Analyzers = append(allpf.Analyzers, hpf.Analyzers...)
	}
	return allpf, nil
}

// load instantiates all enabled addons.
func (a *Applier) load() (map[string]AddOn, error) {
	addons := map[string]AddOn{}
	if _, disabledAddons := a.disabledAddons["openebs"]; !disabledAddons {
		obs, err := openebs.New()
		if err != nil {
			return nil, fmt.Errorf("unable to create openebs addon: %w", err)
		}
		addons["openebs"] = obs
	}
	if _, disabledAddons := a.disabledAddons["embeddedclusteroperator"]; !disabledAddons {
		embedoperator, err := embeddedclusteroperator.New()
		if err != nil {
			return nil, fmt.Errorf("unable to create embedded cluster operator addon: %w", err)
		}
		addons["embeddedclusteroperator"] = embedoperator
	}
	if _, disabledAddons := a.disabledAddons["adminconsole"]; !disabledAddons {
		aconsole, err := adminconsole.New("embedded-cluster", a.prompt, a.config)
		if err != nil {
			return nil, fmt.Errorf("unable to create admin console addon: %w", err)
		}
		addons["adminconsole"] = aconsole
	}
	custom, err := custom.New("embedded-cluster", a.disabledAddons)
	if err != nil {
		return nil, fmt.Errorf("unable to create custom addon: %w", err)
	}
	addons["custom"] = custom
	return addons, nil
}

// Versions returns a map with the version of each addon that will be applied.
func (a *Applier) Versions() (map[string]string, error) {
	addons, err := a.load()
	if err != nil {
		return nil, fmt.Errorf("unable to load addons: %w", err)
	}
	versions := map[string]string{}
	for name, addon := range addons {
		version, err := addon.Version()
		if err != nil {
			return nil, fmt.Errorf("unable to get version (%s): %w", name, err)
		}
		for k, v := range version {
			versions[k] = v
		}
	}
	return versions, nil
}

// kubeClient returns a new kubernetes client.
func (a *Applier) kubeClient() (client.Client, error) {
	k8slogger := zap.New(func(o *zap.Options) {
		o.DestWriter = io.Discard
	})
	log.SetLogger(k8slogger)
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to process kubernetes config: %w", err)
	}
	return client.New(cfg, client.Options{})
}

// waitForKubernetes waits until we manage to make a successful connection to the
// Kubernetes API server.
func (a *Applier) waitForKubernetes(ctx context.Context) error {
	loading := pb.Start()
	defer func() {
		loading.Closef("Kubernetes API server is ready")
	}()
	kcli, err := a.kubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kubernetes client: %w", err)
	}
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	counter := 1
	loading.Infof("1/n Waiting for Kubernetes API server to be ready")
	for {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return ctx.Err()
		}
		counter++
		if err := kcli.List(ctx, &corev1.NamespaceList{}); err != nil {
			loading.Infof(
				"%d/n Waiting for Kubernetes API server to be ready.",
				counter,
			)
			continue
		}
		return nil
	}
}

// NewApplier creates a new Applier instance with all addons registered.
func NewApplier(opts ...Option) *Applier {
	applier := &Applier{
		prompt:         true,
		verbose:        true,
		disabledAddons: map[string]bool{},
		config:         v1beta1.ClusterConfig{},
	}
	for _, fn := range opts {
		fn(applier)
	}
	return applier
}
