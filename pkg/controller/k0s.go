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
	"gopkg.in/yaml.v3"
)

// k0s implement the component interface to run etcd
type K0s struct {
	Config config.Config

	supervisor supervisor.Supervisor
	uid        int
	gid        int
	ctx        context.Context
}

// Init initializes k0s
func (k *K0s) Init(_ context.Context) error {
	err := assets.Stage(static.FS(), k.Config.DataDir, "bin/k0s", 0550)
	if err != nil {
		return fmt.Errorf("stage k0s: %w", err)
	}

	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("get current user: %w", err)
	}
	k.uid, _ = strconv.Atoi(currentUser.Uid)
	k.gid, _ = strconv.Atoi(currentUser.Gid)

	err = k.writeConfigFile()
	if err != nil {
		return fmt.Errorf("write k0s config file: %w", err)
	}

	return nil
}

// Start starts k0s
func (k *K0s) Start(ctx context.Context) error {
	k.ctx = ctx

	k.supervisor = supervisor.Supervisor{
		Name:    "k0s",
		BinPath: assets.BinPath("k0s", k.Config.BinDir),
		RunDir:  k.Config.RunDir,
		DataDir: k.Config.DataDir,
		Args: []string{
			"controller",
			"--enable-worker",
			"--no-taints",
			"--data-dir=" + filepath.Join(k.Config.DataDir, "k0s"),
			"--config=" + k.Config.K0sConfigFile,
		},
		UID:           k.uid,
		GID:           k.gid,
		KeepEnvPrefix: true,
	}
	return k.supervisor.Supervise()
}

// Stop stops k0s
func (k *K0s) Stop() error {
	return k.supervisor.Stop()
}

// Ready is the health-check interface
func (k *K0s) Ready() error {
	// TODO
	return nil
}

func (k *K0s) writeConfigFile() error {
	spec := map[string]interface{}{
		"apiVersion": "k0s.k0sproject.io/v1beta1",
		"kind":       "ClusterConfig",
		"metadata": map[string]interface{}{
			"name": "k0s",
		},
		"spec": map[string]interface{}{
			"network": map[string]interface{}{
				"provider": "calico",
			},
			"extensions": map[string]interface{}{
				"storage": map[string]interface{}{
					"type":                         "openebs_local_storage",
					"create_default_storage_class": true,
				},
				"helm": map[string]interface{}{
					"charts": []interface{}{
						map[string]interface{}{
							"name":      "admin-console",
							"namespace": "default",
							"chartname": "http://localhost:10680/static/charts/admin-console-1.98.3.tgz",
							"version":   "1.98.3",
							"values":    "password: password\nminimalRBAC: false\nisHelmManaged: false\nservice:\n  type: ClusterIP",
						},
					},
				},
			},
		},
	}
	b, err := yaml.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshal spec: %w", err)
	}

	err = os.MkdirAll(filepath.Dir(k.Config.K0sConfigFile), 0755)
	if err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(k.Config.K0sConfigFile), err)
	}

	err = os.WriteFile(k.Config.K0sConfigFile, b, 0644)
	if err != nil {
		return fmt.Errorf("write file %s: %w", k.Config.K0sConfigFile, err)
	}
	return nil
}
