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

	ecv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"sigs.k8s.io/yaml"
)

// SerializeSpec serialize the provided spec inside a HostPreflight object and
// returns the byte slice.
func SerializeSpec(spec *troubleshootv1beta2.HostPreflightSpec) ([]byte, error) {
	hpf := map[string]interface{}{
		"apiVersion": "troubleshoot.sh/v1beta2",
		"kind":       "HostPreflight",
		"metadata":   map[string]interface{}{"name": "embedded-cluster"},
		"spec":       spec,
	}
	return yaml.Marshal(hpf)
}

// Run runs the provided host preflight spec locally. This function is meant to be
// used when upgrading a local node.
func Run(ctx context.Context, spec *troubleshootv1beta2.HostPreflightSpec, proxy *ecv1beta1.ProxySpec) (*Output, string, error) {
	// Deduplicate collectors and analyzers before running preflights
	spec.Collectors = dedup(spec.Collectors)
	spec.Analyzers = dedup(spec.Analyzers)

	fpath, err := saveHostPreflightFile(spec)
	if err != nil {
		return nil, "", fmt.Errorf("unable to save preflight locally: %w", err)
	}
	defer os.Remove(fpath)
	binpath := defaults.PathToEmbeddedClusterBinary("kubectl-preflight")
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	cmd := exec.Command(binpath, "--interactive=false", "--format=json", fpath)
	cmd.Env = os.Environ()
	cmd.Env = proxyEnv(cmd.Env, proxy)
	cmd.Env = pathEnv(cmd.Env)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err = cmd.Run(); err == nil {
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

// saveHostPreflightFile saves the provided spec to a temporary file and returns
// the path to the file. The spec is wrapped in a HostPreflight object before being
// encoded and saved.
func saveHostPreflightFile(spec *troubleshootv1beta2.HostPreflightSpec) (string, error) {
	tmpfile, err := os.CreateTemp("", "troubleshoot-*.yaml")
	if err != nil {
		return "", fmt.Errorf("unable to create temporary file: %w", err)
	}
	defer tmpfile.Close()
	if data, err := SerializeSpec(spec); err != nil {
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
		case "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy", "PATH":
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

func pathEnv(env []string) []string {
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
		next = append(next, fmt.Sprintf("PATH=%s:%s", path, defaults.EmbeddedClusterBinsSubDir()))
	} else {
		next = append(next, fmt.Sprintf("PATH=%s", defaults.EmbeddedClusterBinsSubDir()))
	}
	return next
}
