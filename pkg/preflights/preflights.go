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
	"runtime"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig"
	rexec "github.com/k0sproject/rig/exec"
	"github.com/k0sproject/rig/log"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/conversion"
	"sigs.k8s.io/yaml"

	"github.com/replicatedhq/helmvm/pkg/defaults"
	pb "github.com/replicatedhq/helmvm/pkg/progressbar"
)

// UnserlizeSpec unserializes an HostPreflightSpec from a raw slice of bytes.
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
		"metadata":   map[string]interface{}{"name": "helmvm"},
		"spec":       spec,
	}
	return yaml.Marshal(hpf)
}

// RunLocal runs the provided host preflight spec locally. This function is meant to be
// used when upgrading a local node.
func RunLocal(ctx context.Context, spec *v1beta2.HostPreflightSpec) (*Output, error) {
	if runtime.GOOS != "linux" {
		return nil, errors.New("local preflights are only supported on linux hosts")
	}
	stop, err := startProgressbar("localhost")
	if err != nil {
		return nil, fmt.Errorf("unable to start running host preflight: %w", err)
	}
	defer stop()
	fpath, err := saveHostPreflightFile(spec)
	if err != nil {
		return nil, fmt.Errorf("unable to save preflight locally: %w", err)
	}
	defer os.Remove(fpath)
	binpath := defaults.PathToHelmVMBinary("preflight")
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

// Run runs the provided host preflight spec on the provided host.
func Run(ctx context.Context, host *cluster.Host, spec *v1beta2.HostPreflightSpec) (*Output, error) {
	stop, err := startProgressbar(host.Address())
	if err != nil {
		return nil, fmt.Errorf("unable to start running host preflight: %w", err)
	}
	defer stop()
	fpath, err := saveHostPreflightFile(spec)
	if err != nil {
		return nil, err
	}
	defer os.Remove(fpath)
	if err := uploadFile(host, fpath, "/tmp/helmvm-preflight.yaml", 0600); err != nil {
		return nil, err
	}
	binpath := defaults.PathToHelmVMBinary("preflight")
	if err := uploadFile(host, binpath, "/tmp/preflight", 0755); err != nil {
		return nil, err
	}
	return execute(host)
}

// startProgressbar starts a progress bar for the provided host. Returns a function that
// must be called stop the progress bar. Redirects rig's logger to a log file.
func startProgressbar(addr string) (func(), error) {
	fpath := defaults.PathToLog(fmt.Sprintf("helmvm-preflight-%s.log", addr))
	fp, err := os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("unable to open log file: %w", err)
	}
	logfile := logrus.New()
	logfile.SetLevel(logrus.DebugLevel)
	logfile.SetOutput(fp)
	loading := pb.Start()
	loading.Infof("Running host preflight on %s", addr)
	orig := log.Log
	rig.SetLogger(logfile)
	return func() {
		loading.Closef("Host Preflight checks completed on host %s", addr)
		log.Log = orig
		fp.Close()
	}, nil
}

// execute executes the host preflight remotely on the provided host. for troubleshoot exit
// codes see https://troubleshoot.sh/docs/preflight/exit-codes.
func execute(host *cluster.Host) (*Output, error) {
	if err := host.Connect(); err != nil {
		return nil, fmt.Errorf("unable to connect to host: %w", err)
	}
	defer host.Disconnect()
	arg := "--interactive=false --format=json"
	cmd := fmt.Sprintf("/tmp/preflight %s /tmp/helmvm-preflight.yaml 2>/dev/null", arg)
	var err error
	out := bytes.NewBuffer(nil)
	opts := []rexec.Option{rexec.Sudo(host), rexec.Writer(out)}
	if err = host.Connection.Exec(cmd, opts...); err == nil {
		return OutputFromReader(out)
	}
	var exit *ssh.ExitError
	if !errors.As(err, &exit) || exit.ExitStatus() < 2 {
		return nil, fmt.Errorf("unable to run host preflight: %w", err)
	}
	return OutputFromReader(out)
}

// uploadFile uploads a file from src to the host and stores it at dst with the provided
// permissions set.
func uploadFile(host *cluster.Host, src, dst string, mode os.FileMode) error {
	if err := host.Connect(); err != nil {
		return fmt.Errorf("unable to connect to host: %w", err)
	}
	defer host.Disconnect()
	if err := host.Connection.Upload(src, dst); err != nil {
		return fmt.Errorf("unable to upload file: %w", err)
	}
	cmd := fmt.Sprintf("chmod %o %s", mode, dst)
	if _, err := host.Connection.ExecOutput(cmd); err != nil {
		return fmt.Errorf("unable to change file mode: %w", err)
	}
	return nil
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
