// Package preflights manages running host preflights on remote hosts.
package preflights

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
func Run(ctx context.Context, spec *troubleshootv1beta2.HostPreflightSpec, proxy *ecv1beta1.ProxySpec) (*Output, error) {
	// Deduplicate collectors and analyzers before running preflights
	spec.Collectors = dedup(spec.Collectors)
	spec.Analyzers = dedup(spec.Analyzers)

	fpath, err := saveHostPreflightFile(spec)
	if err != nil {
		return nil, fmt.Errorf("unable to save preflight locally: %w", err)
	}
	defer os.Remove(fpath)
	binpath := defaults.PathToEmbeddedClusterBinary("kubectl-preflight")
	stdout := bytes.NewBuffer(nil)
	cmd := exec.Command(binpath, "--interactive=false", "--format=json", fpath)
	cmd.Env = proxyEnv(proxy)
	cmd.Stdout, cmd.Stderr = stdout, io.Discard
	if err = cmd.Run(); err == nil {
		return OutputFromReader(stdout)
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() < 2 {
		return nil, fmt.Errorf("unknown error running host preflight: %w", err)
	}
	return OutputFromReader(stdout)
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

func proxyEnv(proxy *ecv1beta1.ProxySpec) []string {
	env := []string{}
	for _, e := range os.Environ() {
		switch strings.SplitN(e, "=", 2)[0] {
		// Unset proxy environment variables
		case "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy":
			continue
		}
		env = append(env, e)
	}
	if proxy != nil {
		env = append(env, fmt.Sprintf("HTTP_PROXY=%s", proxy.HTTPProxy))
		env = append(env, fmt.Sprintf("HTTPS_PROXY=%s", proxy.HTTPSProxy))
		env = append(env, fmt.Sprintf("NO_PROXY=%s", proxy.NoProxy))
	}
	return env
}
