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
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart"
	helmcli "helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	k8syaml "sigs.k8s.io/yaml"
)

var _ Client = (*HelmClient)(nil)

func newClient(opts HelmOptions) (*HelmClient, error) {
	tmpdir, err := os.MkdirTemp(os.TempDir(), "helm-cache-*")
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
		executor:              newBinaryExecutor(opts.HelmPath),
		tmpdir:                tmpdir,
		kversion:              kversion,
		kubernetesEnvSettings: opts.KubernetesEnvSettings,
		airgapPath:            opts.AirgapPath,
		repositories:          []*repo.Entry{},
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
	helmPath              string               // Path to helm binary
	executor              BinaryExecutor       // Mockable executor
	tmpdir                string               // Temporary directory for helm
	kversion              *semver.Version      // Kubernetes version for template rendering
	kubernetesEnvSettings *helmcli.EnvSettings // Kubernetes environment settings
	airgapPath            string               // Airgap path where charts are stored
	repositories          []*repo.Entry        // Repository entries for helm repo commands
}

func (h *HelmClient) prepare(ctx context.Context) error {
	// Update all repositories to ensure we have the latest chart information
	for _, repo := range h.repositories {
		args := []string{"repo", "update", repo.Name}
		_, _, err := h.executor.ExecuteCommand(ctx, nil, nil, args...)
		if err != nil {
			return fmt.Errorf("helm repo update %s: %w", repo.Name, err)
		}
	}
	return nil
}

func (h *HelmClient) Close() error {
	return os.RemoveAll(h.tmpdir)
}

func (h *HelmClient) AddRepo(ctx context.Context, repo *repo.Entry) error {
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
	// Update repositories if this is not an OCI chart
	if !isOCIChart(ref) {
		if err := h.prepare(ctx); err != nil {
			return "", fmt.Errorf("prepare: %w", err)
		}
	}

	// Use helm pull to download the chart
	args := []string{"pull", ref}
	if version != "" {
		args = append(args, "--version", version)
	}
	args = append(args, "--destination", h.tmpdir)

	// Add debug flag to report progress and capture debug logs
	args = append(args, "--debug")

	_, _, err := h.executor.ExecuteCommand(ctx, nil, nil, args...)
	if err != nil {
		return "", fmt.Errorf("helm pull: %w", err)
	}

	// Get chart metadata to determine the actual chart name and construct filename
	metadata, err := h.GetChartMetadata(ctx, ref, version)
	if err != nil {
		return "", fmt.Errorf("get chart metadata: %w", err)
	}

	// Construct expected filename (chart name + version + .tgz)
	chartPath := filepath.Join(h.tmpdir, fmt.Sprintf("%s-%s.tgz", metadata.Name, metadata.Version))

	return chartPath, nil
}

func (h *HelmClient) RegistryAuth(ctx context.Context, server, user, pass string) error {
	// Use helm registry login for authentication
	args := []string{"registry", "login", server, "--username", user, "--password", pass}

	_, _, err := h.executor.ExecuteCommand(ctx, nil, nil, args...)
	if err != nil {
		return fmt.Errorf("helm registry login: %w", err)
	}

	return nil
}

func (h *HelmClient) Push(ctx context.Context, path, dst string) error {
	// Use helm push to upload the chart
	args := []string{"push", path, dst}

	_, _, err := h.executor.ExecuteCommand(ctx, nil, nil, args...)
	if err != nil {
		return fmt.Errorf("helm push: %w", err)
	}

	return nil
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

// ReleaseHistoryEntry represents a single entry in helm release history
type ReleaseHistoryEntry struct {
	Revision int            `json:"revision"`
	Status   release.Status `json:"status"`
}

// ReleaseHistory returns the release history for a given release
func (h *HelmClient) ReleaseHistory(ctx context.Context, namespace string, releaseName string, maxRevisions int) ([]ReleaseHistoryEntry, error) {
	args := []string{"history", releaseName, "--namespace", namespace, "--output", "json"}

	if maxRevisions > 0 {
		args = append(args, "--max", fmt.Sprintf("%d", maxRevisions))
	}

	// Add kubeconfig and context if available
	args = h.addKubernetesEnvArgs(args)

	stdout, _, err := h.executor.ExecuteCommand(ctx, nil, nil, args...)
	if err != nil {
		return nil, fmt.Errorf("helm history: %w", err)
	}

	var history []ReleaseHistoryEntry
	if err := json.Unmarshal([]byte(stdout), &history); err != nil {
		return nil, fmt.Errorf("parse release history json: %w", err)
	}

	return history, nil
}

// GetLastRevision returns the revision number of the latest release entry
func (h *HelmClient) GetLastRevision(ctx context.Context, namespace string, releaseName string) (int, error) {
	history, err := h.ReleaseHistory(ctx, namespace, releaseName, 1)
	if err != nil {
		return 0, fmt.Errorf("get release history: %w", err)
	}

	if len(history) == 0 {
		return 0, fmt.Errorf("no release history found for %s", releaseName)
	}

	return history[0].Revision, nil
}

func (h *HelmClient) ReleaseExists(ctx context.Context, namespace string, releaseName string) (bool, error) {
	history, err := h.ReleaseHistory(ctx, namespace, releaseName, 1)
	if err != nil {
		if strings.Contains(err.Error(), "release: not found") {
			return false, nil
		}
		return false, fmt.Errorf("get release history: %w", err)
	}

	// True if release has history and is not uninstalled
	exists := len(history) > 0 && history[0].Status != release.StatusUninstalled

	return exists, nil
}

// createValuesFile creates a temporary values file from the provided values map
func (h *HelmClient) createValuesFile(values map[string]interface{}) (string, error) {
	if h.tmpdir == "" {
		return "", fmt.Errorf("tmpdir not initialized")
	}

	cleanVals, err := cleanUpGenericMap(values)
	if err != nil {
		return "", fmt.Errorf("clean up generic map: %w", err)
	}

	data, err := k8syaml.Marshal(cleanVals)
	if err != nil {
		return "", fmt.Errorf("marshal values: %w", err)
	}

	// Use unique filename to prevent race conditions
	valuesFile := filepath.Join(h.tmpdir, fmt.Sprintf("values-%d.yaml", time.Now().UnixNano()))
	if err := os.WriteFile(valuesFile, data, 0644); err != nil {
		return "", fmt.Errorf("write values file: %w", err)
	}

	return valuesFile, nil
}

func (h *HelmClient) Install(ctx context.Context, opts InstallOptions) (string, error) {
	// Build helm install command arguments
	args := []string{"install", opts.ReleaseName}

	// Handle chart source
	chartPath, err := h.resolveChartPath(ctx, opts.ReleaseName, opts.ChartPath, opts.ChartVersion)
	if err != nil {
		return "", fmt.Errorf("resolve chart path: %w", err)
	}
	args = append(args, chartPath)

	// Add namespace
	if opts.Namespace != "" {
		args = append(args, "--namespace", opts.Namespace)
		args = append(args, "--create-namespace")
	}

	// Add wait options
	args = append(args, "--wait")
	args = append(args, "--wait-for-jobs")

	// Add timeout
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	args = append(args, "--timeout", timeout.String())

	// Add replace flag
	args = append(args, "--replace")

	// Add values if provided
	if opts.Values != nil {
		valuesFile, err := h.createValuesFile(opts.Values)
		if err != nil {
			return "", fmt.Errorf("create values file: %w", err)
		}
		defer os.Remove(valuesFile)
		args = append(args, "--values", valuesFile)
	}

	// Add labels if provided
	if opts.Labels != nil {
		var labelPairs []string
		for k, v := range opts.Labels {
			labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", k, v))
		}
		args = append(args, "--labels", strings.Join(labelPairs, ","))
	}

	// Add kubeconfig and context if available
	args = h.addKubernetesEnvArgs(args)

	// Add debug flag to report progress and capture debug logs
	args = append(args, "--debug")

	// NOTE: we don't set client.Atomic = true on install as it makes installation failures difficult to debug
	// since it will rollback the release.

	// Execute helm install command
	stdout, _, err := h.executor.ExecuteCommand(ctx, nil, opts.LogFn, args...)
	if err != nil {
		return "", fmt.Errorf("execute: %w", err)
	}

	return stdout, nil
}

// resolveChartPath handles chart source resolution for install, upgrade, and render operations
func (h *HelmClient) resolveChartPath(ctx context.Context, releaseName, chartPath, chartVersion string) (string, error) {
	if h.airgapPath != "" {
		// Use chart from airgap path
		return filepath.Join(h.airgapPath, fmt.Sprintf("%s-%s.tgz", releaseName, chartVersion)), nil
	}
	if !strings.HasPrefix(chartPath, "/") {
		// Pull chart with retries (includes oci:// prefix)
		localPath, err := h.PullByRefWithRetries(ctx, chartPath, chartVersion, 3)
		if err != nil {
			return "", fmt.Errorf("pull chart: %w", err)
		}
		if localPath == "" {
			return "", fmt.Errorf("pulled chart path is empty")
		}
		return localPath, nil
	}
	// Use local chart path
	return chartPath, nil
}

func (h *HelmClient) Upgrade(ctx context.Context, opts UpgradeOptions) (string, error) {
	// Build helm upgrade command arguments
	args := []string{"upgrade", opts.ReleaseName}

	// Handle chart source
	chartPath, err := h.resolveChartPath(ctx, opts.ReleaseName, opts.ChartPath, opts.ChartVersion)
	if err != nil {
		return "", fmt.Errorf("resolve chart path: %w", err)
	}
	args = append(args, chartPath)

	// Add namespace
	if opts.Namespace != "" {
		args = append(args, "--namespace", opts.Namespace)
	}

	// Add wait options
	args = append(args, "--wait")
	args = append(args, "--wait-for-jobs")

	// Add timeout
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	args = append(args, "--timeout", timeout.String())

	// Add atomic flag
	args = append(args, "--atomic")

	// Add force flag if specified
	if opts.Force {
		args = append(args, "--force")
	}

	// Add values if provided
	if opts.Values != nil {
		valuesFile, err := h.createValuesFile(opts.Values)
		if err != nil {
			return "", fmt.Errorf("create values file: %w", err)
		}
		defer os.Remove(valuesFile)
		args = append(args, "--values", valuesFile)
	}

	// Add labels if provided
	if opts.Labels != nil {
		var labelPairs []string
		for k, v := range opts.Labels {
			labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", k, v))
		}
		args = append(args, "--labels", strings.Join(labelPairs, ","))
	}

	// Add kubernetes environment arguments
	args = h.addKubernetesEnvArgs(args)

	// Add debug flag to report progress and capture debug logs
	args = append(args, "--debug")

	// Execute helm upgrade command
	stdout, stderr, err := h.executor.ExecuteCommand(ctx, nil, opts.LogFn, args...)
	if err != nil {
		if shouldRollback(err.Error()) || shouldRollback(stderr) {
			// Get the last revision
			lastRevision, err := h.GetLastRevision(ctx, opts.Namespace, opts.ReleaseName)
			if err != nil {
				return "", fmt.Errorf("get last revision: %w", err)
			}

			// Rollback to the latest revision
			if _, err := h.Rollback(ctx, RollbackOptions{
				ReleaseName: opts.ReleaseName,
				Namespace:   opts.Namespace,
				Revision:    lastRevision,
				Timeout:     opts.Timeout,
				Force:       opts.Force,
				LogFn:       opts.LogFn,
			}); err != nil {
				return "", fmt.Errorf("rollback: %w", err)
			}

			// Retry upgrade after successful rollback
			return h.Upgrade(ctx, opts)
		}

		return "", fmt.Errorf("helm upgrade failed: %w", err)
	}

	return stdout, nil
}

func shouldRollback(err string) bool {
	return strings.Contains(err, "another operation") && strings.Contains(err, "in progress")
}

func (h *HelmClient) Rollback(ctx context.Context, opts RollbackOptions) (string, error) {
	args := []string{"rollback", opts.ReleaseName}

	// If specific revision is provided, use it
	if opts.Revision > 0 {
		args = append(args, fmt.Sprintf("%d", opts.Revision))
	}

	// Add namespace
	if opts.Namespace != "" {
		args = append(args, "--namespace", opts.Namespace)
	}

	// Add wait options
	args = append(args, "--wait")
	args = append(args, "--wait-for-jobs")

	// Add timeout
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	args = append(args, "--timeout", timeout.String())

	// Add force flag if specified
	if opts.Force {
		args = append(args, "--force")
	}

	// Add kubernetes environment arguments
	args = h.addKubernetesEnvArgs(args)

	// Add debug flag to report progress and capture debug logs
	args = append(args, "--debug")

	stdout, _, err := h.executor.ExecuteCommand(ctx, nil, opts.LogFn, args...)
	if err != nil {
		return "", fmt.Errorf("execute: %w", err)
	}

	return stdout, nil
}

func (h *HelmClient) Uninstall(ctx context.Context, opts UninstallOptions) error {
	// Build helm uninstall command arguments
	args := []string{"uninstall", opts.ReleaseName}

	// Add namespace
	if opts.Namespace != "" {
		args = append(args, "--namespace", opts.Namespace)
	}

	// Add wait flag
	if opts.Wait {
		args = append(args, "--wait")
	}

	// Add ignore not found flag
	if opts.IgnoreNotFound {
		args = append(args, "--ignore-not-found")
	}

	// Add kubeconfig and context if available
	args = h.addKubernetesEnvArgs(args)

	// Add debug flag to report progress and capture debug logs
	args = append(args, "--debug")

	// Add timeout from context if available
	if deadline, ok := ctx.Deadline(); ok {
		timeout := time.Until(deadline)
		args = append(args, "--timeout", timeout.String())
	}

	// Execute helm uninstall command
	_, _, err := h.executor.ExecuteCommand(ctx, nil, opts.LogFn, args...)
	if err != nil {
		return fmt.Errorf("execute: %w", err)
	}

	return nil
}

func (h *HelmClient) Render(ctx context.Context, opts InstallOptions) ([][]byte, error) {
	// Build helm template command arguments
	args := []string{"template", opts.ReleaseName}

	// Handle chart source
	chartPath, err := h.resolveChartPath(ctx, opts.ReleaseName, opts.ChartPath, opts.ChartVersion)
	if err != nil {
		return nil, fmt.Errorf("resolve chart path: %w", err)
	}
	args = append(args, chartPath)

	// Add namespace
	if opts.Namespace != "" {
		args = append(args, "--namespace", opts.Namespace)
		args = append(args, "--create-namespace")
	}

	// Add labels if provided
	if opts.Labels != nil {
		var labelPairs []string
		for k, v := range opts.Labels {
			labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", k, v))
		}
		args = append(args, "--labels", strings.Join(labelPairs, ","))
	}

	// Add values if provided
	if opts.Values != nil {
		valuesFile, err := h.createValuesFile(opts.Values)
		if err != nil {
			return nil, fmt.Errorf("create values file: %w", err)
		}
		defer os.Remove(valuesFile)
		args = append(args, "--values", valuesFile)
	}

	// Add kubernetes version if available
	if h.kversion != nil {
		args = append(args, "--kube-version", fmt.Sprintf("v%d.%d.0", h.kversion.Major(), h.kversion.Minor()))
	}

	// Add kubeconfig and context if available
	args = h.addKubernetesEnvArgs(args)

	// Add include CRDs flag
	args = append(args, "--include-crds")

	// Add debug flag to report progress and capture debug logs
	args = append(args, "--debug")

	// Execute helm template command
	stdout, _, err := h.executor.ExecuteCommand(ctx, nil, opts.LogFn, args...)
	if err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}

	manifests, err := splitManifests(stdout)
	if err != nil {
		return nil, fmt.Errorf("parse helm template output: %w", err)
	}
	return manifests, nil
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
