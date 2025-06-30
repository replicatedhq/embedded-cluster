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
	"path/filepath"
	"strings"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"sigs.k8s.io/yaml"
)

// Run runs the provided host preflight spec locally. This function is meant to be
// used when upgrading a local node.
func (p *PreflightsRunner) Run(ctx context.Context, spec *troubleshootv1beta2.HostPreflightSpec, rc runtimeconfig.RuntimeConfig) (*apitypes.HostPreflightsOutput, string, error) {
	// Deduplicate collectors and analyzers before running preflights
	spec.Collectors = dedup(spec.Collectors)
	spec.Analyzers = dedup(spec.Analyzers)

	fpath, err := saveHostPreflightFile(spec)
	if err != nil {
		return nil, "", fmt.Errorf("unable to save preflight locally: %w", err)
	}
	defer os.Remove(fpath)

	binpath := rc.PathToEmbeddedClusterBinary("kubectl-preflight")
	cmd := exec.Command(binpath, "--interactive=false", "--format=json", fpath)

	cmdEnv := cmd.Environ()
	cmdEnv = proxyEnv(cmdEnv, rc.ProxySpec())
	cmdEnv = pathEnv(cmdEnv, rc)
	cmd.Env = cmdEnv

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
		return nil, stderr.String(), fmt.Errorf("error running host preflight: %w, stderr=%q", err, stderr.String())
	}

	out, err := OutputFromReader(stdout)
	return out, stderr.String(), err
}

func (p *PreflightsRunner) CopyBundleTo(dst string) error {
	matches, err := filepath.Glob("preflightbundle-*.tar.gz")
	if err != nil {
		return fmt.Errorf("find preflight bundle: %w", err)
	}
	if len(matches) == 0 {
		return nil
	}
	// get the newest bundle
	src := matches[0]
	for _, match := range matches {
		if filepath.Base(match) > filepath.Base(src) {
			src = match
		}
	}
	if err := helpers.MoveFile(src, dst); err != nil {
		return fmt.Errorf("move preflight bundle to %s: %w", dst, err)
	}
	return nil
}

// serializeSpec serialize the provided spec inside a HostPreflight object and
// returns the byte slice.
func serializeSpec(spec *troubleshootv1beta2.HostPreflightSpec) ([]byte, error) {
	hpf := map[string]interface{}{
		"apiVersion": "troubleshoot.sh/v1beta2",
		"kind":       "HostPreflight",
		"metadata":   map[string]interface{}{"name": "embedded-cluster"},
		"spec":       spec,
	}
	return yaml.Marshal(hpf)
}

// saveHostPreflightFile saves the provided spec to a temporary file and returns
// the path to the file. The spec is wrapped in a HostPreflight object before being
// encoded and saved.
func saveHostPreflightFile(spec *troubleshootv1beta2.HostPreflightSpec) (string, error) {
	tmpfile, err := os.CreateTemp("", "troubleshoot-*.yaml")
	if err != nil {
		return "", fmt.Errorf("unable to create temporary file: %w", err)
	}
	defer tmpfile.Close()
	if data, err := serializeSpec(spec); err != nil {
		return "", fmt.Errorf("unable to serialize host preflight spec: %w", err)
	} else if _, err := tmpfile.Write(data); err != nil {
		return "", fmt.Errorf("unable to write host preflight spec: %w", err)
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

func pathEnv(env []string, rc runtimeconfig.RuntimeConfig) []string {
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
		next = append(next, fmt.Sprintf("PATH=%s:%s", path, rc.EmbeddedClusterBinsSubDir()))
	} else {
		next = append(next, fmt.Sprintf("PATH=%s", rc.EmbeddedClusterBinsSubDir()))
	}
	return next
}
