package controller

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/emosbaugh/helmbin/pkg/assets"
	"github.com/emosbaugh/helmbin/pkg/config"
	"github.com/emosbaugh/helmbin/pkg/supervisor"
	"github.com/emosbaugh/helmbin/static"
)

// K3s implement the component interface to run etcd
type K3s struct {
	Config config.Config

	supervisor supervisor.Supervisor
	uid        int
	gid        int
	ctx        context.Context
}

// Init initializes k3s
func (k *K3s) Init(_ context.Context) error {
	err := assets.Stage(static.FS(), k.Config.DataDir, "bin/k3s", 0550)
	if err != nil {
		return fmt.Errorf("stage k3s: %w", err)
	}

	_ = os.RemoveAll(filepath.Join(k.Config.DataDir, "k3s/server/static"))
	err = assets.Stage(static.FS(), k.Config.DataDir, "k3s/server/static", 0440)
	if err != nil {
		return fmt.Errorf("stage k3s/server/static: %w", err)
	}

	_ = os.RemoveAll(filepath.Join(k.Config.DataDir, "k3s/server/manifests"))
	err = assets.Stage(static.FS(), k.Config.DataDir, "k3s/server/manifests", 0440)
	if err != nil {
		return fmt.Errorf("stage k3s/server/manifests: %w", err)
	}

	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("get current user: %w", err)
	}
	k.uid, _ = strconv.Atoi(currentUser.Uid)
	k.gid, _ = strconv.Atoi(currentUser.Gid)

	return nil
}

// Start starts k3s
func (k *K3s) Start(ctx context.Context) error {
	k.ctx = ctx

	_, err := os.Stat(k.Config.K3sConfigFile)
	if !os.IsNotExist(err) {
		_ = os.Setenv("K3S_CONFIG_FILE", k.Config.K3sConfigFile)
	}

	k.supervisor = supervisor.Supervisor{
		Name:    "k3s",
		BinPath: assets.BinPath("k3s", k.Config.BinDir),
		RunDir:  k.Config.RunDir,
		DataDir: k.Config.DataDir,
		Args: []string{
			"server",
			"--data-dir=" + filepath.Join(k.Config.DataDir, "k3s"),
			fmt.Sprintf("--write-kubeconfig=%s", k.Config.KubeconfigPath),
			"--kubelet-arg=root-dir=" + filepath.Join(k.Config.DataDir, "k3s/kubelet"),
		},
		UID:           k.uid,
		GID:           k.gid,
		KeepEnvPrefix: true,
	}
	return k.supervisor.Supervise()
}

// Stop stops k3s
func (k *K3s) Stop() error {
	return k.supervisor.Stop()
}

// Ready is the health-check interface
func (k *K3s) Ready() error {
	// TODO
	return nil
}
