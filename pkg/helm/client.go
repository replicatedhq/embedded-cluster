package helm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/pusher"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/storage/driver"
	"helm.sh/helm/v3/pkg/uploader"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	k8syaml "sigs.k8s.io/yaml"
)

var (
	// getters is a list of known getters for both http and
	// oci schemes.
	getters = getter.Providers{
		getter.Provider{
			Schemes: []string{"http", "https"},
			New:     getter.NewHTTPGetter,
		},
		getter.Provider{
			Schemes: []string{"oci"},
			New:     getter.NewOCIGetter,
		},
	}

	// pushers holds all supported pushers (uploaders).
	pushers = pusher.Providers{
		pusher.Provider{
			Schemes: []string{"oci"},
			New:     pusher.NewOCIPusher,
		},
	}
)

var _ Client = (*HelmClient)(nil)

func newClient(opts HelmOptions) (*HelmClient, error) {
	tmpdir, err := os.MkdirTemp(os.TempDir(), "helm-cache-*")
	if err != nil {
		return nil, err
	}
	registryOpts := []registry.ClientOption{}
	if opts.Writer != nil {
		registryOpts = append(registryOpts, registry.ClientOptWriter(opts.Writer))
	}
	var kversion *semver.Version
	if opts.K0sVersion != "" {
		sv, err := semver.NewVersion(opts.K0sVersion)
		if err != nil {
			return nil, fmt.Errorf("parse k0s version: %w", err)
		}
		kversion = sv
	}
	regcli, err := registry.NewClient(registryOpts...)
	if err != nil {
		return nil, fmt.Errorf("create registry client: %w", err)
	}
	if opts.RESTClientGetter == nil {
		cfgFlags := &genericclioptions.ConfigFlags{}
		if opts.KubeConfig != "" {
			cfgFlags.KubeConfig = &opts.KubeConfig
		}
		opts.RESTClientGetter = cfgFlags
	}
	return &HelmClient{
		tmpdir:           tmpdir,
		kversion:         kversion,
		restClientGetter: opts.RESTClientGetter,
		regcli:           regcli,
		logFn:            opts.LogFn,
		airgapPath:       opts.AirgapPath,
	}, nil
}

type HelmOptions struct {
	KubeConfig       string
	RESTClientGetter genericclioptions.RESTClientGetter
	K0sVersion       string
	AirgapPath       string
	Writer           io.Writer
	LogFn            action.DebugLog
}

type InstallOptions struct {
	ReleaseName  string
	ChartPath    string
	ChartVersion string
	Values       map[string]interface{}
	Namespace    string
	Labels       map[string]string
	Timeout      time.Duration
}

type UpgradeOptions struct {
	ReleaseName  string
	ChartPath    string
	ChartVersion string
	Values       map[string]interface{}
	Namespace    string
	Labels       map[string]string
	Timeout      time.Duration
	Force        bool
}

type UninstallOptions struct {
	ReleaseName    string
	Namespace      string
	Wait           bool
	IgnoreNotFound bool
}

type HelmClient struct {
	tmpdir           string
	kversion         *semver.Version
	restClientGetter genericclioptions.RESTClientGetter
	regcli           *registry.Client
	repocfg          string
	repos            []*repo.Entry
	reposChanged     bool
	logFn            action.DebugLog
	airgapPath       string
}

func (h *HelmClient) prepare() error {
	// NOTE: this is a hack and should be refactored
	if !h.reposChanged {
		return nil
	}

	data, err := k8syaml.Marshal(repo.File{Repositories: h.repos})
	if err != nil {
		return fmt.Errorf("marshal repositories: %w", err)
	}

	repocfg := filepath.Join(h.tmpdir, "config.yaml")
	if err := os.WriteFile(repocfg, data, 0644); err != nil {
		return fmt.Errorf("write repositories: %w", err)
	}

	for _, repository := range h.repos {
		chrepo, err := repo.NewChartRepository(
			repository, getters,
		)
		if err != nil {
			return fmt.Errorf("create chart repo: %w", err)
		}
		chrepo.CachePath = h.tmpdir
		_, err = chrepo.DownloadIndexFile()
		if err != nil {
			return fmt.Errorf("download index file: %w", err)
		}
	}
	h.repocfg = repocfg
	h.reposChanged = false
	return nil
}

func (h *HelmClient) Close() error {
	return os.RemoveAll(h.tmpdir)
}

func (h *HelmClient) AddRepo(repo *repo.Entry) error {
	h.repos = append(h.repos, repo)
	h.reposChanged = true
	return nil
}

func (h *HelmClient) Latest(reponame, chart string) (string, error) {
	stableConstraint, err := semver.NewConstraint(">0.0.0") // search only for stable versions
	if err != nil {
		return "", fmt.Errorf("create stable constraint: %w", err)
	}

	for _, repository := range h.repos {
		if repository.Name != reponame {
			continue
		}
		chrepo, err := repo.NewChartRepository(repository, getters)
		if err != nil {
			return "", fmt.Errorf("create chart repo: %w", err)
		}
		chrepo.CachePath = h.tmpdir
		idx, err := chrepo.DownloadIndexFile()
		if err != nil {
			return "", fmt.Errorf("download index file: %w", err)
		}

		repoidx, err := repo.LoadIndexFile(idx)
		if err != nil {
			return "", fmt.Errorf("load index file: %w", err)
		}

		versions, ok := repoidx.Entries[chart]
		if !ok {
			return "", fmt.Errorf("chart %s not found", chart)
		}

		if len(versions) == 0 {
			return "", fmt.Errorf("chart %s has no versions", chart)
		}

		for _, version := range versions {
			v, err := semver.NewVersion(version.Version)
			if err != nil {
				continue
			}

			if stableConstraint.Check(v) {
				return version.Version, nil
			}
		}

		return "", fmt.Errorf("no stable version found for chart %s", chart)
	}
	return "", fmt.Errorf("repository %s not found", reponame)
}

func (h *HelmClient) PullByRefWithRetries(ctx context.Context, ref string, version string, tries int) (string, error) {
	for i := 0; ; i++ {
		localPath, err := h.PullByRef(ref, version)
		if err == nil {
			return localPath, nil
		}
		logrus.Debugf("Failed to pull %s:%v (%d/%d): %v", ref, version, i+1, tries, err)
		if i == tries-1 {
			return "", err
		}
		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

func (h *HelmClient) Pull(reponame, chart string, version string) (string, error) {
	ref := fmt.Sprintf("%s/%s", reponame, chart)
	return h.PullByRef(ref, version)
}

func (h *HelmClient) PullByRef(ref string, version string) (string, error) {
	if !isOCIChart(ref) {
		if err := h.prepare(); err != nil {
			return "", fmt.Errorf("prepare: %w", err)
		}
	}

	dl := downloader.ChartDownloader{
		Out:              io.Discard,
		Options:          []getter.Option{},
		RepositoryConfig: h.repocfg,
		RepositoryCache:  h.tmpdir,
		Getters:          getters,
	}

	dst, _, err := dl.DownloadTo(ref, version, os.TempDir())
	if err != nil {
		return "", fmt.Errorf("download chart %s: %w", ref, err)
	}

	return dst, nil
}

func (h *HelmClient) RegistryAuth(server, user, pass string) error {
	return h.regcli.Login(server, registry.LoginOptBasicAuth(user, pass))
}

func (h *HelmClient) Push(path, dst string) error {
	up := uploader.ChartUploader{
		Out:     os.Stdout,
		Pushers: pushers,
		Options: []pusher.Option{pusher.WithRegistryClient(h.regcli)},
	}

	return up.UploadTo(path, dst)
}

func (h *HelmClient) GetChartMetadata(chartPath string) (*chart.Metadata, error) {
	chartRequested, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("load chart: %w", err)
	}

	return chartRequested.Metadata, nil
}

// reference: https://github.com/helm/helm/blob/0d66425d9a745d8a289b1a5ebb6ccc744436da95/cmd/helm/upgrade.go#L122-L125
func (h *HelmClient) ReleaseExists(ctx context.Context, namespace string, releaseName string) (bool, error) {
	cfg, err := h.getActionCfg(namespace)
	if err != nil {
		return false, fmt.Errorf("get action configuration: %w", err)
	}

	client := action.NewHistory(cfg)
	client.Max = 1

	versions, err := client.Run(releaseName)
	if errors.Is(err, driver.ErrReleaseNotFound) || isReleaseUninstalled(versions) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get release history: %w", err)
	}

	return true, nil
}

func isReleaseUninstalled(versions []*release.Release) bool {
	return len(versions) > 0 && versions[len(versions)-1].Info.Status == release.StatusUninstalled
}

func (h *HelmClient) Install(ctx context.Context, opts InstallOptions) (*release.Release, error) {
	cfg, err := h.getActionCfg(opts.Namespace)
	if err != nil {
		return nil, fmt.Errorf("get action configuration: %w", err)
	}

	client := action.NewInstall(cfg)
	client.ReleaseName = opts.ReleaseName
	client.Namespace = opts.Namespace
	client.Labels = opts.Labels
	client.Replace = true
	client.CreateNamespace = true
	client.WaitForJobs = true
	client.Wait = true
	// we don't set client.Atomic = true on install as it makes installation failures difficult to
	// debug since it will rollback the release.

	if opts.Timeout != 0 {
		client.Timeout = opts.Timeout
	} else {
		client.Timeout = 5 * time.Minute
	}

	chartRequested, err := h.loadChart(ctx, opts.ReleaseName, opts.ChartPath, opts.ChartVersion)
	if err != nil {
		return nil, fmt.Errorf("load chart: %w", err)
	}

	if req := chartRequested.Metadata.Dependencies; req != nil {
		if err := action.CheckDependencies(chartRequested, req); err != nil {
			return nil, fmt.Errorf("check chart dependencies: %w", err)
		}
	}

	cleanVals, err := cleanUpGenericMap(opts.Values)
	if err != nil {
		return nil, fmt.Errorf("clean up generic map: %w", err)
	}

	release, err := client.RunWithContext(ctx, chartRequested, cleanVals)
	if err != nil {
		return nil, fmt.Errorf("helm install: %w", err)
	}

	return release, nil
}

func (h *HelmClient) Upgrade(ctx context.Context, opts UpgradeOptions) (*release.Release, error) {
	cfg, err := h.getActionCfg(opts.Namespace)
	if err != nil {
		return nil, fmt.Errorf("get action configuration: %w", err)
	}

	client := action.NewUpgrade(cfg)
	client.Namespace = opts.Namespace
	client.Labels = opts.Labels
	client.WaitForJobs = true
	client.Wait = true
	client.Atomic = true
	client.Force = opts.Force

	if opts.Timeout != 0 {
		client.Timeout = opts.Timeout
	} else {
		client.Timeout = 5 * time.Minute
	}

	chartRequested, err := h.loadChart(ctx, opts.ReleaseName, opts.ChartPath, opts.ChartVersion)
	if err != nil {
		return nil, fmt.Errorf("load chart: %w", err)
	}

	if req := chartRequested.Metadata.Dependencies; req != nil {
		if err := action.CheckDependencies(chartRequested, req); err != nil {
			return nil, fmt.Errorf("check chart dependencies: %w", err)
		}
	}

	cleanVals, err := cleanUpGenericMap(opts.Values)
	if err != nil {
		return nil, fmt.Errorf("clean up generic map: %w", err)
	}

	release, err := client.RunWithContext(ctx, opts.ReleaseName, chartRequested, cleanVals)
	if err != nil {
		return nil, fmt.Errorf("helm upgrade: %w", err)
	}

	return release, nil
}

func (h *HelmClient) Uninstall(ctx context.Context, opts UninstallOptions) error {
	cfg, err := h.getActionCfg(opts.Namespace)
	if err != nil {
		return fmt.Errorf("get action configuration: %w", err)
	}

	client := action.NewUninstall(cfg)
	client.Wait = opts.Wait
	client.IgnoreNotFound = opts.IgnoreNotFound

	if deadline, ok := ctx.Deadline(); ok {
		client.Timeout = time.Until(deadline)
	}

	if _, err := client.Run(opts.ReleaseName); err != nil {
		return fmt.Errorf("uninstall release: %w", err)
	}

	return nil
}

func (h *HelmClient) Render(ctx context.Context, opts InstallOptions) ([][]byte, error) {
	cfg := &action.Configuration{}

	client := action.NewInstall(cfg)
	client.DryRun = true
	client.ReleaseName = opts.ReleaseName
	client.Replace = true
	client.CreateNamespace = true
	client.ClientOnly = true
	client.IncludeCRDs = true
	client.Namespace = opts.Namespace
	client.Labels = opts.Labels

	if h.kversion != nil {
		// since ClientOnly is true we need to initialize KubeVersion otherwise resorts defaults
		client.KubeVersion = &chartutil.KubeVersion{
			Version: fmt.Sprintf("v%d.%d.0", h.kversion.Major(), h.kversion.Minor()),
			Major:   fmt.Sprintf("%d", h.kversion.Major()),
			Minor:   fmt.Sprintf("%d", h.kversion.Minor()),
		}
	}

	chartRequested, err := h.loadChart(ctx, opts.ReleaseName, opts.ChartPath, opts.ChartVersion)
	if err != nil {
		return nil, fmt.Errorf("load chart: %w", err)
	}

	if req := chartRequested.Metadata.Dependencies; req != nil {
		if err := action.CheckDependencies(chartRequested, req); err != nil {
			return nil, fmt.Errorf("failed dependency check: %w", err)
		}
	}

	cleanVals, err := cleanUpGenericMap(opts.Values)
	if err != nil {
		return nil, fmt.Errorf("clean up generic map: %w", err)
	}

	release, err := client.Run(chartRequested, cleanVals)
	if err != nil {
		return nil, fmt.Errorf("run render: %w", err)
	}

	var manifests bytes.Buffer
	fmt.Fprintln(&manifests, strings.TrimSpace(release.Manifest))
	for _, m := range release.Hooks {
		fmt.Fprintf(&manifests, "---\n# Source: %s\n%s\n", m.Path, m.Manifest)
	}

	resources := [][]byte{}
	splitManifests := releaseutil.SplitManifests(manifests.String())
	for _, manifest := range splitManifests {
		manifest = strings.TrimSpace(manifest)
		resources = append(resources, []byte(manifest))
	}

	return resources, nil
}

func (h *HelmClient) getActionCfg(namespace string) (*action.Configuration, error) {
	cfg := &action.Configuration{}
	var logFn action.DebugLog
	if h.logFn != nil {
		logFn = h.logFn
	} else {
		logFn = _logFn
	}
	restClientGetter := &namespacedRESTClientGetter{
		RESTClientGetter: h.restClientGetter,
		namespace:        namespace,
	}
	if err := cfg.Init(restClientGetter, namespace, "secret", logFn); err != nil {
		return nil, fmt.Errorf("init helm configuration: %w", err)
	}
	return cfg, nil
}

func (h *HelmClient) loadChart(ctx context.Context, releaseName, chartPath, chartVersion string) (*chart.Chart, error) {
	var localPath string
	if h.airgapPath != "" {
		// airgapped, use chart from airgap path
		// TODO: this should just respect the chart path if it's a local path and leave it up to the caller to handle
		localPath = filepath.Join(h.airgapPath, fmt.Sprintf("%s-%s.tgz", releaseName, chartVersion))
	} else if !strings.HasPrefix(chartPath, "/") {
		// Assume this is a chart from a repo if it doesn't start with a /
		// This includes oci:// prefix
		var err error
		localPath, err = h.PullByRefWithRetries(ctx, chartPath, chartVersion, 3)
		if err != nil {
			return nil, fmt.Errorf("pull: %w", err)
		}
		defer os.RemoveAll(localPath)
	} else {
		localPath = chartPath
	}

	chartRequested, err := loader.Load(localPath)
	if err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}

	return chartRequested, nil
}

func cleanUpGenericMap(m map[string]interface{}) (map[string]interface{}, error) {
	// we must first use yaml marshal to convert the map[interface{}]interface{} to a []byte
	// otherwise we will get an error "unsupported type: map[interface {}]interface {}"
	b, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("yaml marshal: %w", err)
	}
	next := map[string]interface{}{}
	err = k8syaml.Unmarshal(b, &next)
	if err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w", err)
	}
	return next, nil
}

func isOCIChart(chartPath string) bool {
	return strings.HasPrefix(chartPath, "oci://")
}

func _logFn(format string, args ...interface{}) {
	log := logrus.WithField("component", "helm")
	log.Debugf(format, args...)
}

type namespacedRESTClientGetter struct {
	genericclioptions.RESTClientGetter
	namespace string
}

func (n *namespacedRESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	cfg := n.RESTClientGetter.ToRawKubeConfigLoader()
	return &namespacedClientConfig{
		cfg:       cfg,
		namespace: n.namespace,
	}
}

type namespacedClientConfig struct {
	cfg       clientcmd.ClientConfig
	namespace string
}

func (n *namespacedClientConfig) RawConfig() (clientcmdapi.Config, error) {
	return n.cfg.RawConfig()
}

func (n *namespacedClientConfig) ClientConfig() (*restclient.Config, error) {
	return n.cfg.ClientConfig()
}

func (n *namespacedClientConfig) Namespace() (string, bool, error) {
	if n.namespace == "" {
		return n.cfg.Namespace()
	}
	return n.namespace, true, nil
}

func (n *namespacedClientConfig) ConfigAccess() clientcmd.ConfigAccess {
	return n.cfg.ConfigAccess()
}
