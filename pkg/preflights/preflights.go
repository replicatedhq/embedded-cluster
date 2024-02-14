// Package preflights manages running host preflights on remote hosts.
package preflights

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/conversion"
	"sigs.k8s.io/yaml"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
)

// UnserializeSpec unserializes an HostPreflightSpec from a raw slice of bytes.
func UnserializeSpec(data []byte) (*v1beta2.HostPreflightSpec, error) {
	scheme := kruntime.NewScheme()
	if err := v1beta2.AddToScheme(scheme); err != nil {
		return nil, err
	}
	decoder := conversion.NewDecoder(scheme)
	var hpf v1beta2.HostPreflight
	if err := decoder.DecodeInto(data, &hpf); err != nil {
		return nil, err
	}
	return &hpf.Spec, nil
}

// SerializeSpec serialize the provided spec inside a HostPreflight object and
// returns the byte slice.
func SerializeSpec(spec *v1beta2.HostPreflightSpec) ([]byte, error) {
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
func Run(ctx context.Context, spec *v1beta2.HostPreflightSpec) (*Output, error) {
	fpath, err := saveHostPreflightFile(spec)
	if err != nil {
		return nil, fmt.Errorf("unable to save preflight locally: %w", err)
	}
	defer os.Remove(fpath)
	binpath := defaults.PathToEmbeddedClusterBinary("kubectl-preflight")
	stdout := bytes.NewBuffer(nil)
	cmd := exec.Command(binpath, "--interactive=false", "--format=json", fpath)
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
func saveHostPreflightFile(spec *v1beta2.HostPreflightSpec) (string, error) {
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
