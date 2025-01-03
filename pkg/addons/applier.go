// Package addons manages the default addons installations in the cluster. Addons are
// mostly Helm Charts, but can also be other resources as the project evolves. All of
// the AddOns must implement the AddOn interface.
package addons

import (
	"context"
	"fmt"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/kinds/types"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/velero"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

const SpecDataKey = "support-bundle-spec"

// AddOn is the interface that all addons must implement.
type AddOn interface {
	Version() (map[string]string, error)
	Name() string
	HostPreflights() (*v1beta2.HostPreflightSpec, error)
	GenerateHelmConfig(k0sCfg *k0sv1beta1.ClusterConfig, onlyDefaults bool) ([]ecv1beta1.Chart, []k0sv1beta1.Repository, error)
	Outro(ctx context.Context, cli client.Client, k0sCfg *k0sv1beta1.ClusterConfig, releaseMetadata *types.ReleaseMetadata) error
	GetProtectedFields() map[string][]string
	GetImages() []string
	GetAdditionalImages() []string
}

// Applier is an entity that applies (installs and updates) addons in the cluster.
type Applier struct {
	prompt                  bool
	verbose                 bool
	adminConsolePwd         string // admin console password
	license                 *kotsv1beta1.License
	licenseFile             string
	onlyDefaults            bool
	endUserConfig           *ecv1beta1.Config
	airgapBundle            string
	isAirgap                bool
	proxyEnv                map[string]string
	privateCAs              map[string]string
	isHA                    bool
	isHAMigrationInProgress bool
	binaryNameOverride      string
	configValuesFile        string
}

// Outro runs the outro in all enabled add-ons.
func (a *Applier) Outro(ctx context.Context, k0sCfg *k0sv1beta1.ClusterConfig, endUserCfg *ecv1beta1.Config, releaseMetadata *types.ReleaseMetadata, networkInterface string) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}
	addons, err := a.load()
	if err != nil {
		return fmt.Errorf("unable to load addons: %w", err)
	}

	errCh := kubeutils.WaitForKubernetes(ctx, kcli)
	defer func() {
		for len(errCh) > 0 {
			err := <-errCh
			logrus.Error(fmt.Errorf("infrastructure failed to become ready: %w", err))
		}
	}()

	for _, addon := range addons {
		if err := addon.Outro(ctx, kcli, k0sCfg, releaseMetadata); err != nil {
			return err
		}
	}
	if err := spinForInstallation(ctx, kcli); err != nil {
		return err
	}

	err = kotscli.CreateHostSupportBundle()
	if err != nil {
		logrus.Warnf("failed to create host support bundle: %v", err)
	}

	if err := printKotsadmLinkMessage(a.license, networkInterface, runtimeconfig.AdminConsolePort()); err != nil {
		return fmt.Errorf("unable to print success message: %w", err)
	}

	return nil
}

// OutroForRestore runs the outro in all enabled add-ons for restore operations.
func (a *Applier) OutroForRestore(ctx context.Context, k0sCfg *k0sv1beta1.ClusterConfig) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}
	addons, err := a.loadForRestore()
	if err != nil {
		return fmt.Errorf("unable to load addons: %w", err)
	}
	for _, addon := range addons {
		if err := addon.Outro(ctx, kcli, k0sCfg, nil); err != nil {
			return err
		}
	}
	return nil
}

// GenerateHelmConfigs generates the helm config for all the embedded charts.
func (a *Applier) GenerateHelmConfigs(k0sCfg *k0sv1beta1.ClusterConfig, additionalCharts []ecv1beta1.Chart, additionalRepositories []k0sv1beta1.Repository) ([]ecv1beta1.Chart, []k0sv1beta1.Repository, error) {
	charts := []ecv1beta1.Chart{}
	repositories := []k0sv1beta1.Repository{}
	addons, err := a.load()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to load addons: %w", err)
	}

	// charts required by embedded-cluster
	for _, addon := range addons {
		addonChartConfig, addonRepositoryConfig, err := addon.GenerateHelmConfig(k0sCfg, a.onlyDefaults)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to generate helm config for %s: %w", addon, err)
		}
		charts = append(charts, addonChartConfig...)
		repositories = append(repositories, addonRepositoryConfig...)
	}

	// charts required by the application
	charts = append(charts, additionalCharts...)
	repositories = append(repositories, additionalRepositories...)

	return charts, repositories, nil
}

// GenerateHelmConfigsForRestore generates the helm config for the embedded charts required for a restore operation.
func (a *Applier) GenerateHelmConfigsForRestore(k0sCfg *k0sv1beta1.ClusterConfig) ([]ecv1beta1.Chart, []k0sv1beta1.Repository, error) {
	charts := []ecv1beta1.Chart{}
	repositories := []k0sv1beta1.Repository{}
	addons, err := a.loadForRestore()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to load addons: %w", err)
	}

	// charts required for restore
	for _, addon := range addons {
		addonChartConfig, addonRepositoryConfig, err := addon.GenerateHelmConfig(k0sCfg, a.onlyDefaults)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to generate helm config for %s: %w", addon, err)
		}
		charts = append(charts, addonChartConfig...)
		repositories = append(repositories, addonRepositoryConfig...)
	}

	return charts, repositories, nil
}

// GetBuiltinCharts returns a map of charts that are not applied at install time and instead
// included in metadata for later use by the operator.
func (a *Applier) GetBuiltinCharts(k0sCfg *k0sv1beta1.ClusterConfig) (map[string]ecv1beta1.Helm, error) {
	builtinCharts := map[string]ecv1beta1.Helm{}

	addons, err := a.loadBuiltIn()
	if err != nil {
		return nil, fmt.Errorf("unable to load addons: %w", err)
	}

	for name, addon := range addons {
		chart, repo, err := addon.GenerateHelmConfig(k0sCfg, true)
		if err != nil {
			return nil, fmt.Errorf("unable to generate helm config for %s: %w", name, err)
		}
		builtinCharts[name] = ecv1beta1.Helm{
			Repositories: repo,
			Charts:       chart,
		}
	}

	return builtinCharts, nil
}

func (a *Applier) GetImages() ([]string, error) {
	images := []string{}
	addons, err := a.load()
	if err != nil {
		return nil, fmt.Errorf("unable to load addons: %w", err)
	}
	builtInAddons, err := a.loadBuiltIn()
	if err != nil {
		return nil, fmt.Errorf("unable to load built-in addons: %w", err)
	}
	for _, addon := range addons {
		images = append(images, addon.GetImages()...)
	}
	for _, addon := range builtInAddons {
		images = append(images, addon.GetImages()...)
	}

	return images, nil
}

func (a *Applier) GetAdditionalImages() ([]string, error) {
	images := []string{}
	addons, err := a.load()
	if err != nil {
		return nil, fmt.Errorf("unable to load addons: %w", err)
	}
	builtInAddons, err := a.loadBuiltIn()
	if err != nil {
		return nil, fmt.Errorf("unable to load built-in addons: %w", err)
	}
	for _, addon := range addons {
		images = append(images, addon.GetAdditionalImages()...)
	}
	for _, addon := range builtInAddons {
		images = append(images, addon.GetAdditionalImages()...)
	}

	return images, nil
}

// ProtectedFields returns the protected fields for all the embedded charts.
func (a *Applier) ProtectedFields() (map[string][]string, error) {
	protectedFields := map[string][]string{}
	addons, err := a.load()
	if err != nil {
		return protectedFields, fmt.Errorf("unable to load addons: %w", err)
	}
	for _, addon := range addons {
		for k, v := range addon.GetProtectedFields() {
			protectedFields[k] = v
		}
	}
	return protectedFields, nil
}

// HostPreflights reads all embedded host preflights from all add-ons and returns them
// merged in a single HostPreflightSpec.
func (a *Applier) HostPreflights() (*v1beta2.HostPreflightSpec, error) {
	addons, err := a.load()
	if err != nil {
		return nil, fmt.Errorf("unable to load addons: %w", err)
	}
	return a.hostPreflights(addons)
}

// HostPreflightsForRestore reads all embedded host preflights from all add-ons for restore operations
// and returns them merged in a single HostPreflightSpec for restore operations.
func (a *Applier) HostPreflightsForRestore() (*v1beta2.HostPreflightSpec, error) {
	addons, err := a.loadForRestore()
	if err != nil {
		return nil, fmt.Errorf("unable to load addons: %w", err)
	}
	return a.hostPreflights(addons)
}

func (a *Applier) hostPreflights(addons []AddOn) (*v1beta2.HostPreflightSpec, error) {
	allpf := &v1beta2.HostPreflightSpec{}
	for _, addon := range addons {
		hpf, err := addon.HostPreflights()
		if err != nil {
			return nil, fmt.Errorf("unable to get preflights for %s: %w", addon.Name(), err)
		} else if hpf == nil {
			continue
		}
		allpf.Collectors = append(allpf.Collectors, hpf.Collectors...)
		allpf.Analyzers = append(allpf.Analyzers, hpf.Analyzers...)
	}
	return allpf, nil
}

// load instantiates and returns all addon appliers.
func (a *Applier) load() ([]AddOn, error) {
	addons := []AddOn{}
	obs, err := openebs.New()
	if err != nil {
		return nil, fmt.Errorf("unable to create openebs addon: %w", err)
	}
	addons = append(addons, obs)

	reg, err := registry.New(runtimeconfig.RegistryNamespace, a.airgapBundle != "" || a.isAirgap, a.isHA, a.isHAMigrationInProgress)
	if err != nil {
		return nil, fmt.Errorf("unable to create registry addon: %w", err)
	}
	addons = append(addons, reg)
	sea, err := seaweedfs.New(runtimeconfig.SeaweedFSNamespace, a.airgapBundle != "" || a.isAirgap, a.isHA)
	if err != nil {
		return nil, fmt.Errorf("unable to create seaweedfs addon: %w", err)
	}
	addons = append(addons, sea)

	embedoperator, err := embeddedclusteroperator.New(
		a.endUserConfig,
		a.license,
		a.airgapBundle != "" || a.isAirgap,
		a.proxyEnv,
		a.privateCAs,
		a.binaryNameOverride,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to create embedded cluster operator addon: %w", err)
	}
	addons = append(addons, embedoperator)

	disasterRecoveryEnabled, err := helpers.DisasterRecoveryEnabled(a.license)
	if err != nil {
		return nil, fmt.Errorf("unable to check if disaster recovery is enabled: %w", err)
	}
	vel, err := velero.New(runtimeconfig.VeleroNamespace, disasterRecoveryEnabled, a.proxyEnv)
	if err != nil {
		return nil, fmt.Errorf("unable to create velero addon: %w", err)
	}
	addons = append(addons, vel)

	aconsole, err := adminconsole.New(
		runtimeconfig.KotsadmNamespace,
		a.adminConsolePwd,
		a.licenseFile,
		a.airgapBundle,
		a.isAirgap,
		a.isHA,
		a.proxyEnv,
		a.privateCAs,
		a.configValuesFile,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to create admin console addon: %w", err)
	}
	addons = append(addons, aconsole)
	return addons, nil
}

// load instantiates and returns all addon appliers.
func (a *Applier) loadBuiltIn() (map[string]AddOn, error) {
	addons := map[string]AddOn{}

	vel, err := velero.New(runtimeconfig.VeleroNamespace, true, a.proxyEnv)
	if err != nil {
		return nil, fmt.Errorf("unable to create velero addon: %w", err)
	}
	addons["velero"] = vel

	reg, err := registry.New(runtimeconfig.RegistryNamespace, true, false, false)
	if err != nil {
		return nil, fmt.Errorf("unable to create registry addon: %w", err)
	}
	addons["registry"] = reg

	regHA, err := registry.New(runtimeconfig.RegistryNamespace, true, true, false)
	if err != nil {
		return nil, fmt.Errorf("unable to create registry addon: %w", err)
	}
	addons["registry-ha"] = regHA

	seaweed, err := seaweedfs.New(runtimeconfig.SeaweedFSNamespace, true, true)
	if err != nil {
		return nil, fmt.Errorf("unable to create seaweedfs addon: %w", err)
	}
	addons["seaweedfs"] = seaweed

	return addons, nil
}

// loadForRestore instantiates and returns addon appliers for restore operations.
func (a *Applier) loadForRestore() ([]AddOn, error) {
	addons := []AddOn{}
	obs, err := openebs.New()
	if err != nil {
		return nil, fmt.Errorf("unable to create openebs addon: %w", err)
	}
	addons = append(addons, obs)

	vel, err := velero.New(runtimeconfig.VeleroNamespace, true, a.proxyEnv)
	if err != nil {
		return nil, fmt.Errorf("unable to create velero addon: %w", err)
	}
	addons = append(addons, vel)
	return addons, nil
}

// Versions returns a map with the version of each addon that will be applied.
func (a *Applier) Versions(additionalCharts []ecv1beta1.Chart) (map[string]string, error) {
	addons, err := a.load()
	if err != nil {
		return nil, fmt.Errorf("unable to load addons: %w", err)
	}

	versions := map[string]string{}
	for _, addon := range addons {
		version, err := addon.Version()
		if err != nil {
			return nil, fmt.Errorf("unable to get version (%s): %w", addon.Name(), err)
		}
		for k, v := range version {
			versions[k] = v
		}
	}

	for _, chart := range additionalCharts {
		versions[chart.Name] = chart.Version
	}

	return versions, nil
}

func spinForInstallation(ctx context.Context, cli client.Client) error {
	installSpin := spinner.Start()
	installSpin.Infof("Waiting for additional components to be ready")

	err := kubeutils.WaitForInstallation(ctx, cli, installSpin)
	if err != nil {
		installSpin.CloseWithError()
		return fmt.Errorf("unable to wait for installation to be ready: %w", err)
	}
	installSpin.Closef("Additional components are ready!")
	return nil
}

// printKotsadmLinkMessage prints the success message when the admin console is online.
func printKotsadmLinkMessage(license *kotsv1beta1.License, networkInterface string, adminConsolePort int) error {
	adminConsoleURL := adminconsole.GetURL(networkInterface, adminConsolePort)

	successColor := "\033[32m"
	colorReset := "\033[0m"
	var successMessage string
	if license != nil {
		successMessage = fmt.Sprintf("Visit the Admin Console to configure and install %s: %s%s%s",
			license.Spec.AppSlug, successColor, adminConsoleURL, colorReset,
		)
	} else {
		successMessage = fmt.Sprintf("Visit the Admin Console to configure and install your application: %s%s%s",
			successColor, adminConsoleURL, colorReset,
		)
	}
	logrus.Info(successMessage)

	return nil
}

// NewApplier creates a new Applier instance with all addons registered.
func NewApplier(opts ...Option) *Applier {
	applier := &Applier{
		prompt:       true,
		verbose:      true,
		airgapBundle: "",
	}
	for _, fn := range opts {
		fn(applier)
	}
	return applier
}
