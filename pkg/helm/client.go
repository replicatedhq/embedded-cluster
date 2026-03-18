package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"go.yaml.in/yaml/v3"
	helmcli "helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/repo"
	chartv4 "helm.sh/helm/v4/pkg/chart/v2"
	k8syaml "sigs.k8s.io/yaml"
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

	return &HelmClient{
		executor:              newBinaryExecutor(opts.HelmPath, tmpdir),
		tmpdir:                tmpdir,
		kversion:              kversion,
		kubernetesEnvSettings: opts.KubernetesEnvSettings,
		airgapPath:            opts.AirgapPath,
	}, nil
}

type HelmOptions struct {
	HelmPath              string // Required: Path to the helm binary
	KubernetesEnvSettings *helmcli.EnvSettings
	K8sVersion            string
	AirgapPath            string
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
	executor              BinaryExecutor       // Mockable executor
	tmpdir                string               // Temporary directory for helm
	kversion              *semver.Version      // Kubernetes version for template rendering
	kubernetesEnvSettings *helmcli.EnvSettings // Kubernetes environment settings
	airgapPath            string               // Airgap path where charts are stored
}


func (h *HelmClient) Close() error {
	return os.RemoveAll(h.tmpdir)
}

func (h *HelmClient) AddRepo(ctx context.Context, r *repo.Entry) error {
	return h.AddRepoBin(ctx, r)
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
	pullDir, err := os.MkdirTemp(h.tmpdir, "pull-*")
	if err != nil {
		return "", fmt.Errorf("create pull dir: %w", err)
	}

	args := []string{"pull", ref, "--destination", pullDir}
	if version != "" {
		args = append(args, "--version", version)
	}

	_, _, err = h.executor.ExecuteCommand(ctx, nil, nil, args...)
	if err != nil {
		os.RemoveAll(pullDir)
		return "", fmt.Errorf("helm pull %s: %w", ref, err)
	}

	matches, err := filepath.Glob(filepath.Join(pullDir, "*.tgz"))
	if err != nil || len(matches) == 0 {
		os.RemoveAll(pullDir)
		return "", fmt.Errorf("no chart archive found after helm pull for %s", ref)
	}
	return matches[0], nil
}

func (h *HelmClient) RegistryAuth(ctx context.Context, server, user, pass string) error {
	// Helm 4 requires domain-only (no https:// or http:// prefix)
	server = strings.TrimPrefix(server, "https://")
	server = strings.TrimPrefix(server, "http://")

	args := []string{"registry", "login", server, "-u", user, "-p", pass}
	_, _, err := h.executor.ExecuteCommand(ctx, nil, nil, args...)
	if err != nil {
		return fmt.Errorf("helm registry login: %w", err)
	}
	return nil
}

func (h *HelmClient) Push(ctx context.Context, path, dst string) error {
	_, _, err := h.executor.ExecuteCommand(ctx, nil, nil, "push", path, dst)
	if err != nil {
		return fmt.Errorf("helm push: %w", err)
	}
	return nil
}

func (h *HelmClient) GetChartMetadata(ctx context.Context, ref string, version string) (*chartv4.Metadata, error) {
	// Use helm show chart to get chart metadata
	args := []string{"show", "chart", ref}
	if version != "" {
		args = append(args, "--version", version)
	}

	stdout, _, err := h.executor.ExecuteCommand(ctx, nil, nil, args...)
	if err != nil {
		return nil, fmt.Errorf("helm show chart: %w", err)
	}

	var metadata chartv4.Metadata
	if err := k8syaml.Unmarshal([]byte(stdout), &metadata); err != nil {
		return nil, fmt.Errorf("parse chart metadata YAML: %w", err)
	}
	return &metadata, nil
}

func (h *HelmClient) ReleaseExists(ctx context.Context, namespace string, releaseName string) (bool, error) {
	args := []string{
		"list",
		"--namespace", namespace,
		"--filter", fmt.Sprintf("^%s$", releaseName),
		"--output", "json",
		"--all", // include uninstalling status
	}
	args = h.addKubernetesEnvArgs(args)

	stdout, _, err := h.executor.ExecuteCommand(ctx, nil, nil, args...)
	if err != nil {
		return false, fmt.Errorf("helm list: %w", err)
	}

	var releases []struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(stdout), &releases); err != nil {
		return false, fmt.Errorf("parse helm list output: %w", err)
	}
	if len(releases) == 0 {
		return false, nil
	}
	// A release is considered to not exist if it's uninstalling (in progress) or
	// uninstalled (completed but kept in history via --keep-history). Callers
	// should install (not upgrade) in both cases.
	status := releases[len(releases)-1].Status
	return status != "uninstalling" && status != "uninstalled", nil
}

func (h *HelmClient) Install(ctx context.Context, opts InstallOptions) (*ReleaseInfo, error) {
	valuesFile, err := h.writeValuesToTemp(opts.Values)
	if err != nil {
		return nil, fmt.Errorf("write values: %w", err)
	}
	if valuesFile != "" {
		defer os.Remove(valuesFile)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	chartPath, cleanup, err := h.resolveChartPath(ctx, opts.ReleaseName, opts.ChartPath, opts.ChartVersion)
	if err != nil {
		return nil, fmt.Errorf("resolve chart: %w", err)
	}
	defer cleanup()

	args := []string{
		"install", opts.ReleaseName, chartPath,
		"--namespace", opts.Namespace,
		"--create-namespace",
		"--wait",
		"--wait-for-jobs",
		"--timeout", timeout.String(),
		"--replace",
		"--output", "json",
	}
	if valuesFile != "" {
		args = append(args, "--values", valuesFile)
	}
	for k, v := range opts.Labels {
		args = append(args, "--labels", fmt.Sprintf("%s=%s", k, v))
	}
	args = h.addKubernetesEnvArgs(args)

	stdout, _, err := h.executor.ExecuteCommand(ctx, nil, opts.LogFn, args...)
	if err != nil {
		return nil, fmt.Errorf("helm install: %w", err)
	}
	return parseReleaseOutput(stdout)
}

func (h *HelmClient) Upgrade(ctx context.Context, opts UpgradeOptions) (*ReleaseInfo, error) {
	valuesFile, err := h.writeValuesToTemp(opts.Values)
	if err != nil {
		return nil, fmt.Errorf("write values: %w", err)
	}
	if valuesFile != "" {
		defer os.Remove(valuesFile)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	chartPath, cleanup, err := h.resolveChartPath(ctx, opts.ReleaseName, opts.ChartPath, opts.ChartVersion)
	if err != nil {
		return nil, fmt.Errorf("resolve chart: %w", err)
	}
	defer cleanup()

	args := []string{
		"upgrade", opts.ReleaseName, chartPath,
		"--namespace", opts.Namespace,
		"--wait",
		"--wait-for-jobs",
		"--timeout", timeout.String(),
		"--rollback-on-failure", // Helm 4: replaces --atomic
		"--output", "json",
	}
	if opts.Force {
		args = append(args, "--force-replace") // Helm 4: replaces --force
	}
	if valuesFile != "" {
		args = append(args, "--values", valuesFile)
	}
	for k, v := range opts.Labels {
		args = append(args, "--labels", fmt.Sprintf("%s=%s", k, v))
	}
	args = h.addKubernetesEnvArgs(args)

	stdout, _, err := h.executor.ExecuteCommand(ctx, nil, opts.LogFn, args...)
	if err != nil {
		return nil, fmt.Errorf("helm upgrade: %w", err)
	}
	return parseReleaseOutput(stdout)
}

func (h *HelmClient) Uninstall(ctx context.Context, opts UninstallOptions) error {
	args := []string{"uninstall", opts.ReleaseName, "--namespace", opts.Namespace}
	if opts.Wait {
		args = append(args, "--wait")
	}
	if opts.IgnoreNotFound {
		args = append(args, "--ignore-not-found")
	}
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 {
			args = append(args, "--timeout", remaining.String())
		}
	}
	args = h.addKubernetesEnvArgs(args)

	_, _, err := h.executor.ExecuteCommand(ctx, nil, opts.LogFn, args...)
	if err != nil {
		return fmt.Errorf("helm uninstall: %w", err)
	}
	return nil
}

func (h *HelmClient) Render(ctx context.Context, opts InstallOptions) ([][]byte, error) {
	valuesFile, err := h.writeValuesToTemp(opts.Values)
	if err != nil {
		return nil, fmt.Errorf("write values: %w", err)
	}
	if valuesFile != "" {
		defer os.Remove(valuesFile)
	}

	chartPath, cleanup, err := h.resolveChartPath(ctx, opts.ReleaseName, opts.ChartPath, opts.ChartVersion)
	if err != nil {
		return nil, fmt.Errorf("resolve chart: %w", err)
	}
	defer cleanup()

	args := []string{
		"template", opts.ReleaseName, chartPath,
		"--namespace", opts.Namespace,
		"--include-crds",
	}
	if h.kversion != nil {
		args = append(args, "--kube-version",
			fmt.Sprintf("%d.%d", h.kversion.Major(), h.kversion.Minor()))
	}
	if valuesFile != "" {
		args = append(args, "--values", valuesFile)
	}
	for k, v := range opts.Labels {
		args = append(args, "--labels", fmt.Sprintf("%s=%s", k, v))
	}

	stdout, _, err := h.executor.ExecuteCommand(ctx, nil, opts.LogFn, args...)
	if err != nil {
		return nil, fmt.Errorf("helm template: %w", err)
	}
	return splitManifests(stdout)
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

// writeValuesToTemp writes helm values to a temporary YAML file in h.tmpdir.
// Returns empty string if values is nil/empty. Caller should defer os.Remove(path).
func (h *HelmClient) writeValuesToTemp(values map[string]interface{}) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	cleanVals, err := cleanUpGenericMap(values)
	if err != nil {
		return "", fmt.Errorf("clean values: %w", err)
	}
	data, err := k8syaml.Marshal(cleanVals)
	if err != nil {
		return "", fmt.Errorf("marshal values: %w", err)
	}
	tmpFile, err := os.CreateTemp(h.tmpdir, "values-*.yaml")
	if err != nil {
		return "", fmt.Errorf("create values file: %w", err)
	}
	defer tmpFile.Close()
	if _, err := tmpFile.Write(data); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("write values: %w", err)
	}
	return tmpFile.Name(), nil
}

// resolveChartPath determines the local filesystem path for a chart.
// Returns the path, a cleanup function (call after helm binary finishes), and error.
// The cleanup function is safe to call even on error paths.
func (h *HelmClient) resolveChartPath(ctx context.Context, releaseName, chartPath, chartVersion string) (string, func(), error) {
	noop := func() {}

	if _, err := os.Stat(chartPath); err == nil {
		return chartPath, noop, nil
	}
	if h.airgapPath != "" {
		p := filepath.Join(h.airgapPath, fmt.Sprintf("%s-%s.tgz", releaseName, chartVersion))
		return p, noop, nil
	}
	if !strings.HasPrefix(chartPath, "/") {
		// Treat as repo ref or OCI ref — pull it to a temp dir
		localPath, err := h.PullByRefWithRetries(ctx, chartPath, chartVersion, 3)
		if err != nil {
			return "", noop, fmt.Errorf("pull chart: %w", err)
		}
		// Return cleanup that removes the pull directory (parent of the .tgz file)
		pullDir := filepath.Dir(localPath)
		return localPath, func() { os.RemoveAll(pullDir) }, nil
	}
	return "", noop, fmt.Errorf("chart path not found: %s", chartPath)
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
