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
		logFn:                 opts.LogFn,
	}, nil
}

type HelmOptions struct {
	HelmPath              string // Required: Path to the helm binary
	KubernetesEnvSettings *helmcli.EnvSettings
	K8sVersion            string
	AirgapPath            string
	LogFn                 LogFn // Global log function to use for all commands
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

type HelmClient struct {
	helmPath              string               // Path to helm binary
	executor              BinaryExecutor       // Mockable executor
	tmpdir                string               // Temporary directory for helm
	kversion              *semver.Version      // Kubernetes version for template rendering
	kubernetesEnvSettings *helmcli.EnvSettings // Kubernetes environment settings
	airgapPath            string               // Airgap path where charts are stored
	repositories          []*repo.Entry        // Repository entries for helm repo commands
	logFn                 LogFn                // Global log function to use for all commands
}

func (h *HelmClient) prepare(ctx context.Context) error {
	// Update all repositories to ensure we have the latest chart information
	for _, repo := range h.repositories {
		args := []string{"repo", "update", repo.Name}
		_, stderr, err := h.executor.ExecuteCommand(ctx, nil, h.logFn, args...)
		if err != nil {
			return fmt.Errorf("helm repo update %s: %w, stderr: %s", repo.Name, err, stderr)
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

	_, stderr, err := h.executor.ExecuteCommand(ctx, nil, h.logFn, args...)
	if err != nil {
		return fmt.Errorf("helm repo add: %w, stderr: %s", err, stderr)
	}

	// Store the repository entry for future reference
	h.repositories = append(h.repositories, repo)
	return nil
}

func (h *HelmClient) Latest(ctx context.Context, reponame, chart string) (string, error) {
	// Use helm search repo with JSON output to find the latest version
	args := []string{"search", "repo", fmt.Sprintf("%s/%s", reponame, chart), "--version", ">0.0.0", "--versions", "--output", "json"}

	stdout, stderr, err := h.executor.ExecuteCommand(ctx, nil, h.logFn, args...)
	if err != nil {
		return "", fmt.Errorf("helm search repo: %w, stderr: %s", err, stderr)
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

	_, stderr, err := h.executor.ExecuteCommand(ctx, nil, h.logFn, args...)
	if err != nil {
		return "", fmt.Errorf("helm pull: %w, stderr: %s", err, stderr)
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

	_, stderr, err := h.executor.ExecuteCommand(ctx, nil, h.logFn, args...)
	if err != nil {
		return fmt.Errorf("helm registry login: %w, stderr: %s", err, stderr)
	}

	return nil
}

func (h *HelmClient) Push(ctx context.Context, path, dst string) error {
	// Use helm push to upload the chart
	args := []string{"push", path, dst}

	_, stderr, err := h.executor.ExecuteCommand(ctx, nil, h.logFn, args...)
	if err != nil {
		return fmt.Errorf("helm push: %w, stderr: %s", err, stderr)
	}

	return nil
}

func (h *HelmClient) GetChartMetadata(ctx context.Context, ref string, version string) (*chart.Metadata, error) {
	// Use helm show chart to get chart metadata
	args := []string{"show", "chart", ref}
	if version != "" {
		args = append(args, "--version", version)
	}

	stdout, stderr, err := h.executor.ExecuteCommand(ctx, nil, h.logFn, args...)
	if err != nil {
		return nil, fmt.Errorf("helm show chart: %w, stderr: %s", err, stderr)
	}

	var metadata chart.Metadata
	if err := k8syaml.Unmarshal([]byte(stdout), &metadata); err != nil {
		return nil, fmt.Errorf("parse chart metadata YAML: %w", err)
	}
	return &metadata, nil
}

func (h *HelmClient) ReleaseExists(ctx context.Context, namespace string, releaseName string) (bool, error) {
	// Use helm list to check if release exists
	args := []string{"list", "--namespace", namespace, "--filter", fmt.Sprintf("^%s$", releaseName), "--output", "json"}

	stdout, stderr, err := h.executor.ExecuteCommand(ctx, nil, h.logFn, args...)
	if err != nil {
		return false, fmt.Errorf("helm list: %w, stderr: %s", err, stderr)
	}

	var releases []struct {
		Status release.Status `json:"status"`
	}
	if err := json.Unmarshal([]byte(stdout), &releases); err != nil {
		return false, fmt.Errorf("parse release list JSON: %w", err)
	}

	// True if release exists and is not uninstalled
	exists := len(releases) > 0 && releases[len(releases)-1].Status != release.StatusUninstalled

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

	// Check for log function override
	logFn := h.logFn
	if opts.LogFn != nil {
		logFn = opts.LogFn
	}

	// Execute helm install command
	stdout, stderr, err := h.executor.ExecuteCommand(ctx, nil, logFn, args...)
	if err != nil {
		return "", fmt.Errorf("helm install: %w, stderr: %s", err, stderr)
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

	// Check for log function override
	logFn := h.logFn
	if opts.LogFn != nil {
		logFn = opts.LogFn
	}

	// Execute helm upgrade command
	stdout, stderr, err := h.executor.ExecuteCommand(ctx, nil, logFn, args...)
	if err != nil {
		return "", fmt.Errorf("helm upgrade: %w, stderr: %s", err, stderr)
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

	// Check for log function override
	logFn := h.logFn
	if opts.LogFn != nil {
		logFn = opts.LogFn
	}

	// Execute helm uninstall command
	_, stderr, err := h.executor.ExecuteCommand(ctx, nil, logFn, args...)
	if err != nil {
		return fmt.Errorf("helm uninstall: %w, stderr: %s", err, stderr)
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

	// Check for log function override
	logFn := h.logFn
	if opts.LogFn != nil {
		logFn = opts.LogFn
	}

	// Execute helm template command
	stdout, stderr, err := h.executor.ExecuteCommand(ctx, nil, logFn, args...)
	if err != nil {
		return nil, fmt.Errorf("helm template: %w, stderr: %s", err, stderr)
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
	// Based on addKubernetesCLIFlags function
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
