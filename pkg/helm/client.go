package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"go.yaml.in/yaml/v3"
	"helm.sh/helm/v3/pkg/chart"
	helmcli "helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
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
		helmPath:              opts.HelmPath,
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
	Writer                io.Writer // kept for API compatibility, not used
}

type LogFn func(format string, args ...any)

type InstallOptions struct {
	ReleaseName  string
	ChartPath    string
	ChartVersion string
	Values       map[string]any
	Namespace    string
	Labels       map[string]string
	Timeout      time.Duration
	LogFn        LogFn // Log function override to use for install command
}

type UpgradeOptions struct {
	ReleaseName  string
	ChartPath    string
	ChartVersion string
	Values       map[string]any
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
	airgapPath            string               // Airgap path where charts are stored
}

func (h *HelmClient) Close() error {
	return os.RemoveAll(h.tmpdir)
}

func (h *HelmClient) AddRepo(ctx context.Context, entry *repo.Entry) error {
	return h.AddRepoBin(ctx, entry)
}

// AddRepoBin adds a repository to the helm client using the helm binary. This is necessary because
// the AddRepo method does not work with other methods using the binary executor.
func (h *HelmClient) AddRepoBin(ctx context.Context, entry *repo.Entry) error {
	args := []string{"repo", "add", entry.Name, entry.URL}

	if entry.Username != "" {
		args = append(args, "--username", entry.Username)
	}
	if entry.Password != "" {
		args = append(args, "--password", entry.Password)
	}
	if entry.InsecureSkipTLSverify {
		args = append(args, "--insecure-skip-tls-verify")
	}
	if entry.PassCredentialsAll {
		args = append(args, "--pass-credentials")
	}

	_, _, err := h.executor.ExecuteCommand(ctx, nil, nil, args...)
	if err != nil {
		return fmt.Errorf("helm repo add: %w", err)
	}

	return nil
}

func (h *HelmClient) Latest(ctx context.Context, reponame, chartName string) (string, error) {
	args := []string{"search", "repo", fmt.Sprintf("%s/%s", reponame, chartName), "--version", ">0.0.0", "--versions", "--output", "json"}

	stdout, _, err := h.executor.ExecuteCommand(ctx, nil, nil, args...)
	if err != nil {
		return "", fmt.Errorf("helm search repo: %w", err)
	}

	var results []struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(stdout), &results); err != nil {
		return "", fmt.Errorf("parse helm search json output: %w", err)
	}

	if len(results) == 0 {
		return "", fmt.Errorf("no charts found for %s/%s", reponame, chartName)
	}

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

func (h *HelmClient) Pull(ctx context.Context, reponame, chartName string, version string) (string, error) {
	ref := fmt.Sprintf("%s/%s", reponame, chartName)
	return h.PullByRef(ctx, ref, version)
}

func (h *HelmClient) PullByRef(ctx context.Context, ref string, version string) (string, error) {
	destDir, err := os.MkdirTemp("", "helm-pull-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	args := []string{"pull", ref, "--destination", destDir}
	if version != "" {
		args = append(args, "--version", version)
	}

	_, _, err = h.executor.ExecuteCommand(ctx, nil, nil, args...)
	if err != nil {
		os.RemoveAll(destDir)
		return "", fmt.Errorf("helm pull %s: %w", ref, err)
	}

	entries, err := os.ReadDir(destDir)
	if err != nil {
		os.RemoveAll(destDir)
		return "", fmt.Errorf("read pull destination dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tgz") {
			pulledChartPath := filepath.Join(destDir, entry.Name())

			outFile, err := os.CreateTemp("", "helm-pull-chart-*.tgz")
			if err != nil {
				os.RemoveAll(destDir)
				return "", fmt.Errorf("create destination chart file: %w", err)
			}
			outPath := outFile.Name()
			if err := outFile.Close(); err != nil {
				os.Remove(outPath)
				os.RemoveAll(destDir)
				return "", fmt.Errorf("close destination chart file: %w", err)
			}
			if err := os.Remove(outPath); err != nil {
				os.RemoveAll(destDir)
				return "", fmt.Errorf("prepare destination chart file: %w", err)
			}

			if err := os.Rename(pulledChartPath, outPath); err != nil {
				os.Remove(outPath)
				os.RemoveAll(destDir)
				return "", fmt.Errorf("move pulled chart to destination path: %w", err)
			}
			if err := os.RemoveAll(destDir); err != nil {
				os.Remove(outPath)
				return "", fmt.Errorf("cleanup pull destination dir: %w", err)
			}

			return outPath, nil
		}
	}

	os.RemoveAll(destDir)
	return "", fmt.Errorf("no .tgz file found after pulling %s", ref)
}

func (h *HelmClient) RegistryAuth(ctx context.Context, server, user, pass string) error {
	// Helm registry login requires domain-only (no scheme prefix)
	server = strings.TrimPrefix(server, "https://")
	server = strings.TrimPrefix(server, "http://")

	args := []string{"registry", "login", server, "--username", user, "--password-stdin"}
	_, _, err := h.executor.ExecuteCommandWithInput(ctx, nil, strings.NewReader(pass+"\n"), nil, args...)
	if err != nil {
		return fmt.Errorf("helm registry login: %w", err)
	}
	return nil
}

func (h *HelmClient) Push(ctx context.Context, path, dst string) error {
	args := []string{"push", path, dst}
	_, _, err := h.executor.ExecuteCommand(ctx, nil, nil, args...)
	if err != nil {
		return fmt.Errorf("helm push: %w", err)
	}
	return nil
}

func (h *HelmClient) GetChartMetadata(ctx context.Context, ref string, version string) (*chart.Metadata, error) {
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

func (h *HelmClient) ReleaseExists(ctx context.Context, namespace string, releaseName string) (bool, error) {
	args := []string{"history", releaseName, "--namespace", namespace, "--max", "1", "--output", "json"}
	args = h.addKubernetesEnvArgs(args)

	stdout, stderr, err := h.executor.ExecuteCommand(ctx, nil, nil, args...)
	if err != nil {
		if strings.Contains(stderr, "release: not found") || strings.Contains(err.Error(), "release: not found") {
			return false, nil
		}
		return false, fmt.Errorf("helm history: %w", err)
	}

	type historyEntry struct {
		Status string `json:"status"`
	}
	var entries []historyEntry
	if err := json.Unmarshal([]byte(stdout), &entries); err != nil {
		return false, fmt.Errorf("parse helm history output: %w", err)
	}

	if len(entries) == 0 {
		return false, nil
	}

	last := entries[len(entries)-1]
	if last.Status == string(release.StatusUninstalled) {
		return false, nil
	}

	return true, nil
}

func (h *HelmClient) Install(ctx context.Context, opts InstallOptions) (*release.Release, error) {
	chartPath, err := h.resolveChartPath(ctx, opts.ReleaseName, opts.ChartPath, opts.ChartVersion)
	if err != nil {
		return nil, fmt.Errorf("resolve chart path: %w", err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	args := []string{
		"install", opts.ReleaseName, chartPath,
		"--namespace", opts.Namespace,
		"--create-namespace",
		"--wait",
		"--wait-for-jobs",
		"--replace",
		"--timeout", formatDuration(timeout),
	}

	if opts.ChartVersion != "" && !isLocalPath(opts.ChartPath) && h.airgapPath == "" {
		args = append(args, "--version", opts.ChartVersion)
	}

	for k, v := range opts.Labels {
		args = append(args, "--labels", fmt.Sprintf("%s=%s", k, v))
	}

	valuesFile, cleanup, err := writeValuesFile(opts.Values)
	if err != nil {
		return nil, fmt.Errorf("write values file: %w", err)
	}
	defer cleanup()

	if valuesFile != "" {
		args = append(args, "--values", valuesFile)
	}

	args = h.addKubernetesEnvArgs(args)

	_, _, err = h.executor.ExecuteCommand(ctx, nil, opts.LogFn, args...)
	if err != nil {
		return nil, fmt.Errorf("helm install: %w", err)
	}

	return nil, nil
}

func (h *HelmClient) Upgrade(ctx context.Context, opts UpgradeOptions) (*release.Release, error) {
	chartPath, err := h.resolveChartPath(ctx, opts.ReleaseName, opts.ChartPath, opts.ChartVersion)
	if err != nil {
		return nil, fmt.Errorf("resolve chart path: %w", err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	args := []string{
		"upgrade", opts.ReleaseName, chartPath,
		"--namespace", opts.Namespace,
		"--wait",
		"--wait-for-jobs",
		"--atomic",
		"--install",
		"--timeout", formatDuration(timeout),
	}

	if opts.ChartVersion != "" && !isLocalPath(opts.ChartPath) && h.airgapPath == "" {
		args = append(args, "--version", opts.ChartVersion)
	}

	if opts.Force {
		args = append(args, "--force")
	}

	for k, v := range opts.Labels {
		args = append(args, "--labels", fmt.Sprintf("%s=%s", k, v))
	}

	valuesFile, cleanup, err := writeValuesFile(opts.Values)
	if err != nil {
		return nil, fmt.Errorf("write values file: %w", err)
	}
	defer cleanup()

	if valuesFile != "" {
		args = append(args, "--values", valuesFile)
	}

	args = h.addKubernetesEnvArgs(args)

	_, _, err = h.executor.ExecuteCommand(ctx, nil, opts.LogFn, args...)
	if err != nil {
		return nil, fmt.Errorf("helm upgrade: %w", err)
	}

	return nil, nil
}

func (h *HelmClient) Uninstall(ctx context.Context, opts UninstallOptions) error {
	args := []string{"uninstall", opts.ReleaseName, "--namespace", opts.Namespace}

	if opts.Wait {
		args = append(args, "--wait")

		if deadline, ok := ctx.Deadline(); ok {
			remaining := time.Until(deadline)
			if remaining > 0 {
				args = append(args, "--timeout", formatDuration(remaining))
			}
		}
	}

	args = h.addKubernetesEnvArgs(args)

	_, stderr, err := h.executor.ExecuteCommand(ctx, nil, opts.LogFn, args...)
	if err != nil {
		if opts.IgnoreNotFound && (strings.Contains(stderr, "release: not found") || strings.Contains(err.Error(), "release: not found")) {
			return nil
		}
		return fmt.Errorf("uninstall release: %w", err)
	}

	return nil
}

func (h *HelmClient) Render(ctx context.Context, opts InstallOptions) ([][]byte, error) {
	chartPath, err := h.resolveChartPath(ctx, opts.ReleaseName, opts.ChartPath, opts.ChartVersion)
	if err != nil {
		return nil, fmt.Errorf("resolve chart path: %w", err)
	}

	args := []string{
		"template", opts.ReleaseName, chartPath,
		"--namespace", opts.Namespace,
		"--include-crds",
	}

	if opts.ChartVersion != "" && !isLocalPath(opts.ChartPath) && h.airgapPath == "" {
		args = append(args, "--version", opts.ChartVersion)
	}

	if h.kversion != nil {
		args = append(args, "--kube-version", fmt.Sprintf("%d.%d", h.kversion.Major(), h.kversion.Minor()))
	}

	for k, v := range opts.Labels {
		args = append(args, "--labels", fmt.Sprintf("%s=%s", k, v))
	}

	valuesFile, cleanup, err := writeValuesFile(opts.Values)
	if err != nil {
		return nil, fmt.Errorf("write values file: %w", err)
	}
	defer cleanup()

	if valuesFile != "" {
		args = append(args, "--values", valuesFile)
	}

	// Do NOT add kubernetes env args for Render - client-only, no k8s needed

	stdout, _, err := h.executor.ExecuteCommand(ctx, nil, nil, args...)
	if err != nil {
		return nil, fmt.Errorf("helm template: %w", err)
	}

	manifests, err := splitManifests(stdout)
	if err != nil {
		return nil, fmt.Errorf("split manifests: %w", err)
	}
	return manifests, nil
}

// resolveChartPath determines the chart path to use for helm commands.
func (h *HelmClient) resolveChartPath(_ context.Context, releaseName, chartPath, chartVersion string) (string, error) {
	if isLocalPath(chartPath) {
		return chartPath, nil
	}
	if h.airgapPath != "" {
		return filepath.Join(h.airgapPath, fmt.Sprintf("%s-%s.tgz", releaseName, chartVersion)), nil
	}
	// Not local, not airgap: pass chartPath directly and let helm pull it (with --version flag added at call site)
	return chartPath, nil
}

// isLocalPath returns true if the given path exists on the filesystem.
func isLocalPath(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// writeValuesFile serializes opts.Values to a temporary YAML file for --values.
// Returns the file path (empty string if values is nil/empty), a cleanup function, and any error.
func writeValuesFile(values map[string]any) (string, func(), error) {
	if len(values) == 0 {
		return "", func() {}, nil
	}

	cleanVals, err := cleanUpGenericMap(values)
	if err != nil {
		return "", func() {}, fmt.Errorf("clean up generic map: %w", err)
	}

	data, err := k8syaml.Marshal(cleanVals)
	if err != nil {
		return "", func() {}, fmt.Errorf("marshal values: %w", err)
	}

	f, err := os.CreateTemp("", "helm-values-*.yaml")
	if err != nil {
		return "", func() {}, fmt.Errorf("create values temp file: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", func() {}, fmt.Errorf("write values temp file: %w", err)
	}
	f.Close()

	cleanup := func() {
		os.Remove(f.Name())
	}

	return f.Name(), cleanup, nil
}

// formatDuration formats a time.Duration as a helm-compatible duration string (e.g. "5m0s").
func formatDuration(d time.Duration) string {
	return d.String()
}

func cleanUpGenericMap(m map[string]any) (map[string]any, error) {
	// we must first use yaml marshal to convert the map[interface{}]interface{} to a []byte
	// otherwise we will get an error "unsupported type: map[interface {}]interface {}"
	b, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("yaml marshal: %w", err)
	}
	next := map[string]any{}
	err = k8syaml.Unmarshal(b, &next)
	if err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w", err)
	}
	return next, nil
}

// addKubernetesEnvArgs adds kubernetes environment arguments to the helm command
func (h *HelmClient) addKubernetesEnvArgs(args []string) []string {
	if h.kubernetesEnvSettings == nil {
		return args
	}

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
