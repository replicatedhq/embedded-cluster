// Package preflights manages running host preflights on remote hosts.
package preflights

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"sigs.k8s.io/yaml"
)

// RunHostPreflights runs the provided host preflight spec locally.
func (p *PreflightRunner) RunHostPreflights(ctx context.Context, spec *troubleshootv1beta2.HostPreflightSpec, opts RunOptions) (*apitypes.PreflightsOutput, string, error) {
	// Deduplicate collectors and analyzers before running preflights
	spec.Collectors = dedup(spec.Collectors)
	spec.Analyzers = dedup(spec.Analyzers)

	specYAML, err := serializeHostSpec(spec)
	if err != nil {
		return nil, "", fmt.Errorf("marshal host preflight spec: %w", err)
	}

	return p.runPreflights(ctx, specYAML, opts)
}

// RunAppPreflights runs the provided app preflight spec locally.
func (p *PreflightRunner) RunAppPreflights(ctx context.Context, spec *troubleshootv1beta2.PreflightSpec, opts RunOptions) (*apitypes.PreflightsOutput, string, error) {
	// Deduplicate collectors and analyzers before running preflights
	spec.Collectors = dedup(spec.Collectors)
	spec.Analyzers = dedup(spec.Analyzers)

	specYAML, err := serializeAppSpec(spec)
	if err != nil {
		return nil, "", fmt.Errorf("marshal app preflight spec: %w", err)
	}

	return p.runPreflights(ctx, specYAML, opts)
}

// runPreflights is the shared logic for running both host and app preflights
func (p *PreflightRunner) runPreflights(_ context.Context, specYAML []byte, opts RunOptions) (*apitypes.PreflightsOutput, string, error) {
	// Write spec to temporary file
	fpath, err := saveSpecToTempFile(specYAML)
	if err != nil {
		return nil, "", err
	}
	defer os.Remove(fpath)

	// Execute preflight command
	cmd := exec.Command(opts.PreflightBinaryPath, "--interactive=false", "--format=json", fpath)
	cmd.Env = preparePreflightEnv(cmd.Environ(), opts)

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	cmd.Stdout, cmd.Stderr = stdout, stderr

	err = cmd.Run()
	if err == nil {
		out, err := OutputFromReader(stdout)
		return out, stderr.String(), err
	}

	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() < 2 {
		return nil, stderr.String(), fmt.Errorf("error running preflight: %w, stderr=%q", err, stderr.String())
	}

	out, err := OutputFromReader(stdout)
	return out, stderr.String(), err
}

// serializeHostSpec serialize the provided spec inside a HostPreflight object and
// returns the byte slice.
func serializeHostSpec(spec *troubleshootv1beta2.HostPreflightSpec) ([]byte, error) {
	hpf := map[string]interface{}{
		"apiVersion": "troubleshoot.sh/v1beta2",
		"kind":       "HostPreflight",
		"metadata":   map[string]interface{}{"name": "embedded-cluster"},
		"spec":       spec,
	}
	return yaml.Marshal(hpf)
}

// serializeAppSpec serialize the provided spec inside a Preflight object and
// returns the byte slice.
func serializeAppSpec(spec *troubleshootv1beta2.PreflightSpec) ([]byte, error) {
	pf := map[string]interface{}{
		"apiVersion": "troubleshoot.sh/v1beta2",
		"kind":       "Preflight",
		"metadata":   map[string]interface{}{"name": "embedded-cluster"},
		"spec":       spec,
	}
	return yaml.Marshal(pf)
}

// saveSpecToTempFile saves the YAML spec to a temporary file and returns the file path
func saveSpecToTempFile(specYAML []byte) (string, error) {
	tmpfile, err := os.CreateTemp("", "troubleshoot-*.yaml")
	if err != nil {
		return "", fmt.Errorf("unable to create temporary file: %w", err)
	}
	defer tmpfile.Close()

	if _, err := tmpfile.Write(specYAML); err != nil {
		os.Remove(tmpfile.Name()) // Clean up on write error
		return "", fmt.Errorf("unable to write preflight spec: %w", err)
	}

	return tmpfile.Name(), nil
}

func dedup[T any](objs []T) []T {
	seen := make(map[string]bool)
	out := []T{}

	if len(objs) == 0 {
		return objs
	}

	for _, o := range objs {
		data, err := json.Marshal(o)
		if err != nil {
			out = append(out, o)
			continue
		}
		key := string(data)
		if _, ok := seen[key]; !ok {
			out = append(out, o)
			seen[key] = true
		}
	}
	return out
}

func proxyEnv(env []string, proxy *ecv1beta1.ProxySpec) []string {
	next := []string{}
	for _, e := range env {
		switch strings.SplitN(e, "=", 2)[0] {
		// Unset proxy environment variables
		case "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy":
		default:
			next = append(next, e)
		}
	}
	if proxy != nil {
		next = append(next, fmt.Sprintf("HTTP_PROXY=%s", proxy.HTTPProxy))
		next = append(next, fmt.Sprintf("HTTPS_PROXY=%s", proxy.HTTPSProxy))
		next = append(next, fmt.Sprintf("NO_PROXY=%s", proxy.NoProxy))
	}
	return next
}

func pathEnv(env []string, extraPaths []string) []string {
	path := ""
	next := []string{}
	for _, e := range env {
		switch strings.SplitN(e, "=", 2)[0] {
		// Unset PATH environment variable
		case "PATH":
			path = strings.SplitN(e, "=", 2)[1]
		default:
			next = append(next, e)
		}
	}
	if path != "" {
		next = append(next, fmt.Sprintf("PATH=%s:%s", path, strings.Join(extraPaths, ":")))
	} else if len(extraPaths) > 0 {
		next = append(next, fmt.Sprintf("PATH=%s", strings.Join(extraPaths, ":")))
	}
	return next
}

// disableAutoUpdateEnv disables the auto-update feature of the preflight binary.
// The troubleshoot binary can auto-update itself, but we want explicit version control in EC.
// Reference: https://github.com/replicatedhq/troubleshoot/blob/73836dc6610746abf2978a2a11c06774a9c48a23/cmd/preflight/cli/root.go#L42-L59
func disableAutoUpdateEnv(env []string) []string {
	return append(env, "PREFLIGHT_AUTO_UPDATE=false")
}

// preparePreflightEnv prepares the environment variables for running the preflight binary.
func preparePreflightEnv(env []string, opts RunOptions) []string {
	env = proxyEnv(env, opts.ProxySpec)
	env = pathEnv(env, opts.ExtraPaths)
	env = disableAutoUpdateEnv(env)

	return env
}
