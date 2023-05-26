package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"

	"k8s.io/client-go/kubernetes"
	kconfig "sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/emosbaugh/helmbin/pkg/assets"
	"github.com/emosbaugh/helmbin/pkg/config"
	"github.com/emosbaugh/helmbin/pkg/supervisor"
	"github.com/emosbaugh/helmbin/static"
)

// K0s implements the component interface to run the k0s controller.
type K0s struct {
	Config            config.Config
	ControllerOptions config.ControllerOptions

	supervisor supervisor.Supervisor
	Output     io.Writer
	uid        int
	gid        int
}

// Init initializes k0s.
func (k *K0s) Init(_ context.Context) error {
	if err := assets.Stage(static.FS(), k.Config.DataDir, "bin/k0s", 0550); err != nil {
		return fmt.Errorf("failed to stage k0s: %w", err)
	}
	usr, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}
	k.uid, _ = strconv.Atoi(usr.Uid)
	k.gid, _ = strconv.Atoi(usr.Gid)
	if err := k.writeConfigFile(); err != nil {
		return fmt.Errorf("failed to write k0s config file: %w", err)
	}
	args := []string{
		"controller",
		fmt.Sprintf("--data-dir=%s", filepath.Join(k.Config.DataDir, "k0s")),
		fmt.Sprintf("--config=%s", k.Config.K0sConfigFile),
	}
	if k.ControllerOptions.EnableWorker {
		args = append(args, "--enable-worker")
	}
	if k.ControllerOptions.NoTaints {
		args = append(args, "--no-taints")
	}
	k.supervisor = supervisor.Supervisor{
		Name:          "k0s",
		UID:           k.uid,
		GID:           k.gid,
		BinPath:       assets.BinPath("k0s", k.Config.BinDir),
		Output:        k.Output,
		RunDir:        k.Config.RunDir,
		DataDir:       k.Config.DataDir,
		KeepEnvPrefix: true,
		Args:          args,
	}
	return nil
}

// Start starts k0s.
func (k *K0s) Start(_ context.Context) error {
	return k.supervisor.Supervise()
}

// Stop stops k0s
func (k *K0s) Stop() error {
	return k.supervisor.Stop()
}

// Ready is the health-check interface.
func (k *K0s) Ready() error {
	kubeconfig := path.Join(k.Config.DataDir, "k0s", "pki", "admin.conf")
	_ = os.Setenv("KUBECONFIG", kubeconfig)
	config, err := kconfig.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to read kubeconfig: %w", err)
	}
	cli, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	res, err := cli.Discovery().RESTClient().Get().AbsPath("/readyz").DoRaw(context.Background())
	if err != nil {
		return fmt.Errorf("failed to reach kubernetes readyz endpoint: %w", err)
	}
	if !bytes.Equal(res, []byte("ok")) {
		return fmt.Errorf("kubernetes readyz endpoint returned %s", res)
	}
	return nil
}

// writeConfigFile writes the k0s config file under Config.K0sConfigFile location.
func (k *K0s) writeConfigFile() error {
	if err := os.MkdirAll(filepath.Dir(k.Config.K0sConfigFile), 0755); err != nil {
		return fmt.Errorf("failed to create dir %s: %w", filepath.Dir(k.Config.K0sConfigFile), err)
	}
	in, err := static.FS().Open("config/k0s.yaml")
	if err != nil {
		return fmt.Errorf("failed to open k0s config template: %w", err)
	}
	defer func() {
		_ = in.Close()
	}()
	out, err := os.OpenFile(k.Config.K0sConfigFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open k0s config file %s: %w", k.Config.K0sConfigFile, err)
	}
	defer func() {
		_ = out.Close()
	}()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("failed to write k0s config file %s: %w", k.Config.K0sConfigFile, err)
	}
	return nil
}
