package helm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"go.yaml.in/yaml/v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	helmcli "helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/pusher"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
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
	tmpdir, err := os.MkdirTemp(os.TempDir(), "helm-*")
	if err != nil {
		return nil, err
	}

	var kversion *semver.Version
	if opts.K8sVersion != "" {
		sv, err := semver.NewVersion(opts.K8sVersion)
		if err != nil {
			return nil, fmt.Errorf("parse k0s version: %w", err)
		}
		kversion = sv
	}

	registryOpts := []registry.ClientOption{}
	if opts.Writer != nil {
		registryOpts = append(registryOpts, registry.ClientOptWriter(opts.Writer))
	}
	regcli, err := registry.NewClient(registryOpts...)
	if err != nil {
		return nil, fmt.Errorf("create registry client: %w", err)
	}

	return &HelmClient{
		helmPath:              opts.HelmPath,
		executor:              newBinaryExecutor(opts.HelmPath, tmpdir),
		tmpdir:                tmpdir,
		kversion:              kversion,
		kubernetesEnvSettings: opts.KubernetesEnvSettings,
		regcli:                regcli,
		airgapPath:            opts.AirgapPath,
		repositories:          []*repo.Entry{},
	}, nil
}

type HelmOptions struct {
	HelmPath              string // Required: Path to the helm binary
	KubernetesEnvSettings *helmcli.EnvSettings
	K8sVersion            string
	AirgapPath            string
	Writer                io.Writer
}

type LogFn func(format string, args ...interface{})

type InstallOptions struct {
	ReleaseName  string
	ChartPath    string
	ChartVersion string
	Values       map[string]interface{}
	Namespace    string
	Labels       map[string]string
	Timeout      time.Duration
	LogFn        LogFn // Log function override to use for install command
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
	LogFn        LogFn // Log function override to use for upgrade command
}

type UninstallOptions struct {
	ReleaseName    string
	Namespace      string
	Wait           bool
	IgnoreNotFound bool
	LogFn          LogFn // Log function override to use for uninstall command
}

type RollbackOptions struct {
	ReleaseName string
	Namespace   string
	Revision    int // Target revision to rollback to, 0 for automatic
	Timeout     time.Duration
	Force       bool
	LogFn       LogFn // Log function override to use for rollback command
}

type HelmClient struct {
	helmPath              string               // Path to helm binary
	executor              BinaryExecutor       // Mockable executor
	tmpdir                string               // Temporary directory for helm
	kversion              *semver.Version      // Kubernetes version for template rendering
	kubernetesEnvSettings *helmcli.EnvSettings // Kubernetes environment settings
	regcli                *registry.Client
	repocfg               string
	repos                 []*repo.Entry
	reposChanged          bool
	airgapPath            string        // Airgap path where charts are stored
	repositories          []*repo.Entry // Repository entries for helm repo commands
}

func (h *HelmClient) prepare(_ context.Context) error {
	// NOTE: this is a hack and should be refactored
	if !h.reposChanged {
		return nil
	}

	data, err := k8syaml.Marshal(repo.File{Repositories: h.repos})
	if err != nil {
		return fmt.Errorf("marshal repositories: %w", err)
	}

	repocfg := filepath.Join(h.tmpdir, "config.yaml")
	if err := helpers.WriteFile(repocfg, data, 0644); err != nil {
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

func (h *HelmClient) AddRepo(_ context.Context, repo *repo.Entry) error {
	h.repos = append(h.repos, repo)
	h.reposChanged = true
	return nil
}

// AddRepoBin adds a repository to the helm client using the helm binary. This is necessary because
// the AddRepo method does not work with other methods using the binary executor.
func (h *HelmClient) AddRepoBin(ctx context.Context, repo *repo.Entry) error {
	// Use helm repo add command to add the repository
	args := []string{"repo", "add", repo.Name, repo.URL}

	// Add username/password if provided
	if repo.Username != "" {
		args = append(args, "--username", repo.Username)
	}
	if repo.Password != "" {
		args = append(args, "--password", repo.Password)
	}

	// Add insecure flag if needed
	if repo.InsecureSkipTLSverify {
		args = append(args, "--insecure-skip-tls-verify")
	}

	// Add pass-credentials flag if needed
	if repo.PassCredentialsAll {
		args = append(args, "--pass-credentials")
	}

	_, _, err := h.executor.ExecuteCommand(ctx, nil, nil, args...)
	if err != nil {
		return fmt.Errorf("helm repo add: %w", err)
	}

	// Store the repository entry for future reference
	h.repositories = append(h.repositories, repo)
	return nil
}

func (h *HelmClient) Latest(ctx context.Context, reponame, chart string) (string, error) {
	// Use helm search repo with JSON output to find the latest version
	args := []string{"search", "repo", fmt.Sprintf("%s/%s", reponame, chart), "--version", ">0.0.0", "--versions", "--output", "json"}

	stdout, _, err := h.executor.ExecuteCommand(ctx, nil, nil, args...)
	if err != nil {
		return "", fmt.Errorf("helm search repo: %w", err)
	}

	// Parse JSON output
	var results []struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(stdout), &results); err != nil {
		return "", fmt.Errorf("parse helm search json output: %w", err)
	}

	if len(results) == 0 {
		return "", fmt.Errorf("no charts found for %s/%s", reponame, chart)
	}

	// Return the version of the first result (latest version due to --versions flag)
	return results[0].Version, nil
}

func (h *HelmClient) PullByRefWithRetries(ctx context.Context, ref string, version string, tries int) (string, error) {
	for i := 0; ; i++ {
		localPath, err := h.PullByRef(ctx, ref, version)
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

func (h *HelmClient) Pull(ctx context.Context, reponame, chart string, version string) (string, error) {
	ref := fmt.Sprintf("%s/%s", reponame, chart)
	return h.PullByRef(ctx, ref, version)
}

func (h *HelmClient) PullByRef(ctx context.Context, ref string, version string) (string, error) {
	if !isOCIChart(ref) {
		if err := h.prepare(ctx); err != nil {
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

func (h *HelmClient) RegistryAuth(_ context.Context, server, user, pass string) error {
	return h.regcli.Login(server, registry.LoginOptBasicAuth(user, pass))
}

func (h *HelmClient) Push(_ context.Context, path, dst string) error {
	up := uploader.ChartUploader{
		Out:     os.Stdout,
		Pushers: pushers,
		Options: []pusher.Option{pusher.WithRegistryClient(h.regcli)},
	}

	return up.UploadTo(path, dst)
}

func (h *HelmClient) GetChartMetadata(ctx context.Context, ref string, version string) (*chart.Metadata, error) {
	// Use helm show chart to get chart metadata
	args := []string{"show", "chart", ref}
	if version != "" {
		args = append(args, "--version", version)
	}

	stdout, _, err := h.executor.ExecuteCommand(ctx, nil, nil, args...)
	if err != nil {
		return nil, fmt.Errorf("helm show chart: %w", err)
	}

	var metadata chart.Metadata
	if err := k8syaml.Unmarshal([]byte(stdout), &metadata); err != nil {
		return nil, fmt.Errorf("parse chart metadata YAML: %w", err)
	}
	return &metadata, nil
}

// reference: https://github.com/helm/helm/blob/0d66425d9a745d8a289b1a5ebb6ccc744436da95/cmd/helm/upgrade.go#L122-L125
func (h *HelmClient) ReleaseExists(ctx context.Context, namespace string, releaseName string) (bool, error) {
	cfg, err := h.getActionCfg(namespace, nil)
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
	cfg, err := h.getActionCfg(opts.Namespace, opts.LogFn)
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
	cfg, err := h.getActionCfg(opts.Namespace, opts.LogFn)
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
	cfg, err := h.getActionCfg(opts.Namespace, opts.LogFn)
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

	splitManifests, err := splitManifests(manifests.String())
	if err != nil {
		return nil, fmt.Errorf("split manifests: %w", err)
	}
	return splitManifests, nil
}

func (h *HelmClient) getActionCfg(namespace string, logFn LogFn) (*action.Configuration, error) {
	cfg := &action.Configuration{}
	if logFn == nil {
		logFn = _logFn
	}
	var restClientGetter genericclioptions.RESTClientGetter
	if h.kubernetesEnvSettings != nil {
		restClientGetter = h.kubernetesEnvSettings.RESTClientGetter()
	} else {
		restClientGetter = helmcli.New().RESTClientGetter() // use the default env settings from helm
	}
	restClientGetter = &namespacedRESTClientGetter{
		RESTClientGetter: restClientGetter,
		namespace:        namespace,
	}
	if err := cfg.Init(restClientGetter, namespace, "secret", action.DebugLog(logFn)); err != nil {
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

// addKubernetesEnvArgs adds kubernetes environment arguments to the helm command
func (h *HelmClient) addKubernetesEnvArgs(args []string) []string {
	if h.kubernetesEnvSettings == nil {
		return args
	}

	// Add all helm CLI flags from kubernetesEnvSettings
	// Based on addKubernetesCLIFlags function below
	if h.kubernetesEnvSettings.KubeConfig != "" {
		args = append(args, "--kubeconfig", h.kubernetesEnvSettings.KubeConfig)
	}
	if h.kubernetesEnvSettings.KubeContext != "" {
		args = append(args, "--kube-context", h.kubernetesEnvSettings.KubeContext)
	}
	if h.kubernetesEnvSettings.KubeToken != "" {
		args = append(args, "--kube-token", h.kubernetesEnvSettings.KubeToken)
	}
	if h.kubernetesEnvSettings.KubeAsUser != "" {
		args = append(args, "--kube-as-user", h.kubernetesEnvSettings.KubeAsUser)
	}
	for _, group := range h.kubernetesEnvSettings.KubeAsGroups {
		args = append(args, "--kube-as-group", group)
	}
	if h.kubernetesEnvSettings.KubeAPIServer != "" {
		args = append(args, "--kube-apiserver", h.kubernetesEnvSettings.KubeAPIServer)
	}
	if h.kubernetesEnvSettings.KubeCaFile != "" {
		args = append(args, "--kube-ca-file", h.kubernetesEnvSettings.KubeCaFile)
	}
	if h.kubernetesEnvSettings.KubeTLSServerName != "" {
		args = append(args, "--kube-tls-server-name", h.kubernetesEnvSettings.KubeTLSServerName)
	}
	if h.kubernetesEnvSettings.KubeInsecureSkipTLSVerify {
		args = append(args, "--kube-insecure-skip-tls-verify")
	}
	if h.kubernetesEnvSettings.BurstLimit != 0 {
		args = append(args, "--burst-limit", fmt.Sprintf("%d", h.kubernetesEnvSettings.BurstLimit))
	}
	if h.kubernetesEnvSettings.QPS != 0 {
		args = append(args, "--qps", fmt.Sprintf("%.2f", h.kubernetesEnvSettings.QPS))
	}

	return args
}

// AddKubernetesCLIFlags adds Kubernetes-related CLI flags to a pflag.FlagSet
// This function is used to configure Kubernetes environment settings
func AddKubernetesCLIFlags(flagSet *pflag.FlagSet, kubernetesEnvSettings *helmcli.EnvSettings) {
	// From helm
	// https://github.com/helm/helm/blob/v3.18.3/pkg/cli/environment.go#L145-L163

	flagSet.StringVar(&kubernetesEnvSettings.KubeConfig, "kubeconfig", "", "Path to the kubeconfig file")
	flagSet.StringVar(&kubernetesEnvSettings.KubeContext, "kube-context", kubernetesEnvSettings.KubeContext, "Name of the kubeconfig context to use")
	flagSet.StringVar(&kubernetesEnvSettings.KubeToken, "kube-token", kubernetesEnvSettings.KubeToken, "Bearer token used for authentication")
	flagSet.StringVar(&kubernetesEnvSettings.KubeAsUser, "kube-as-user", kubernetesEnvSettings.KubeAsUser, "Username to impersonate for the operation")
	flagSet.StringArrayVar(&kubernetesEnvSettings.KubeAsGroups, "kube-as-group", kubernetesEnvSettings.KubeAsGroups, "Group to impersonate for the operation, this flag can be repeated to specify multiple groups.")
	flagSet.StringVar(&kubernetesEnvSettings.KubeAPIServer, "kube-apiserver", kubernetesEnvSettings.KubeAPIServer, "The address and the port for the Kubernetes API server")
	flagSet.StringVar(&kubernetesEnvSettings.KubeCaFile, "kube-ca-file", kubernetesEnvSettings.KubeCaFile, "The certificate authority file for the Kubernetes API server connection")
	flagSet.StringVar(&kubernetesEnvSettings.KubeTLSServerName, "kube-tls-server-name", kubernetesEnvSettings.KubeTLSServerName, "Server name to use for Kubernetes API server certificate validation. If it is not provided, the hostname used to contact the server is used")
	// flagSet.BoolVar(&kubernetesEnvSettings.Debug, "helm-debug", kubernetesEnvSettings.Debug, "enable verbose output")
	flagSet.BoolVar(&kubernetesEnvSettings.KubeInsecureSkipTLSVerify, "kube-insecure-skip-tls-verify", kubernetesEnvSettings.KubeInsecureSkipTLSVerify, "If true, the Kubernetes API server's certificate will not be checked for validity. This will make your HTTPS connections insecure")
	// flagSet.StringVar(&kubernetesEnvSettings.RegistryConfig, "helm-registry-config", kubernetesEnvSettings.RegistryConfig, "Path to the Helm registry config file")
	// flagSet.StringVar(&kubernetesEnvSettings.RepositoryConfig, "helm-repository-config", kubernetesEnvSettings.RepositoryConfig, "Path to the file containing Helm repository names and URLs")
	// flagSet.StringVar(&kubernetesEnvSettings.RepositoryCache, "helm-repository-cache", kubernetesEnvSettings.RepositoryCache, "Path to the directory containing cached Helm repository indexes")
	flagSet.IntVar(&kubernetesEnvSettings.BurstLimit, "burst-limit", kubernetesEnvSettings.BurstLimit, "Kubernetes API client-side default throttling limit")
	flagSet.Float32Var(&kubernetesEnvSettings.QPS, "qps", kubernetesEnvSettings.QPS, "Queries per second used when communicating with the Kubernetes API, not including bursting")
}
