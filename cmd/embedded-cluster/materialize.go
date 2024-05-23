package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/goods"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
)

// materializeContainerdConfig materializes the expected containerd configuration. this is
// useful only when running on airgap environments so this function assess if there is a
// registry running in the cluster and if not bails out without error. this function is
// meant to be ran against a running cluster in both worker or control plane nodes.
func materializeContainerdConfig(c *cli.Context) error {
	cli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	var svc corev1.Service
	nsn := types.NamespacedName{Namespace: "registry", Name: "registry"}
	if err := cli.Get(c.Context, nsn, &svc); err != nil {
		if errors.IsNotFound(err) {
			logrus.Info("registry service not found, skipping containerd config")
			return nil
		}
		return fmt.Errorf("unable to get registry service: %w", err)
	}
	registryAddress := fmt.Sprintf("%s:5000", svc.Spec.ClusterIP)

	parent := defaults.NewProvider(c.String("basedir")).PathToK0sContainerdConfig()
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("unable to create containerd config directory: %w", err)
	}

	contents := airgap.RenderContainerdRegistryConfig(registryAddress)
	path := filepath.Join(parent, "embedded-registry.toml")
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		return fmt.Errorf("unable to write embedded-registry.toml: %w", err)
	}
	return nil
}

var materializeCommand = &cli.Command{
	Name:   "materialize",
	Usage:  "Materialize embedded assets",
	Hidden: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "basedir",
			Usage: "Base directory to materialize assets",
			Value: "",
		},
	},
	Before: func(c *cli.Context) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("materialize command must be run as root")
		}
		return nil
	},
	Action: func(c *cli.Context) error {
		materializer := goods.NewMaterializer(c.String("basedir"))
		if err := materializer.Materialize(); err != nil {
			return fmt.Errorf("unable to materialize: %v", err)
		}
		if err := materializeContainerdConfig(c); err != nil {
			return fmt.Errorf("unable to materialize containerd config: %v", err)
		}
		return nil
	},
}
