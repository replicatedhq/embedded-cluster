package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/tgzutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// These constant define the expected names of the files in the registry.
const (
	EmbeddedClusterBinaryArtifactName = "embedded-cluster-amd64"
	ImagesArtifactName                = "images-amd64.tar"
	HelmChartsArtifactName            = "charts.tar.gz"
)

// kubecli holds a global reference to a Kubernetes client.
var kubecli client.Client

// pullCommand pulls artifacts from the registry running in the cluster and stores
// them locally. This command is used during cluster upgrades when we want to fetch
// the most up to date images, binaries and helm charts.
var pullCommand = &cli.Command{
	Name:  "pull",
	Usage: "Pull artifacts for an airgap installation",
	Before: func(c *cli.Context) (err error) {
		if kubecli, err = kubeutils.KubeClient(); err != nil {
			return fmt.Errorf("unable to create kube client: %w", err)
		}
		return nil
	},
	Subcommands: []*cli.Command{binariesCommand, imagesCommand, helmChartsCommand},
}

// imagesCommand pulls images from the registry running in the cluster and stores
// them locally. This command is used during cluster upgrades when we want to fetch
// the most up to date images. Images are stored in a tarball file in the default
// location.
var imagesCommand = &cli.Command{
	Name:      "images",
	Usage:     "Pull image artifacts for an airgap installation",
	UsageText: `embedded-cluster-operator pull images <installation-name>`,
	Before: func(c *cli.Context) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("pull images command must be run as root")
		}
		if len(c.Args().Slice()) != 1 {
			return fmt.Errorf("expected installation name as argument")
		}
		return nil
	},
	Action: func(c *cli.Context) error {
		in, err := fetchAndValidateInstallation(c.Context, c.Args().First())
		if err != nil {
			return err
		}

		from := in.Spec.Artifacts.Images
		logrus.Infof("fetching images artifact from %s", from)
		location, err := pullArtifact(c.Context, from)
		if err != nil {
			return fmt.Errorf("unable to fetch artifact: %w", err)
		}
		defer func() {
			logrus.Infof("removing temporary directory %s", location)
			os.RemoveAll(location)
		}()

		dst := filepath.Join(defaults.EmbeddedClusterImagesSubDir(), ImagesArtifactName)
		src := filepath.Join(location, ImagesArtifactName)
		logrus.Infof("%s > %s", src, dst)
		if err := helpers.MoveFile(src, dst); err != nil {
			return fmt.Errorf("unable to move images bundle: %w", err)
		}

		logrus.Infof("images materialized under %s", dst)
		return nil
	},
}

// helmChartsCommand pulls helm charts from the registry running in the cluster and
// stores them locally. This command is used during cluster upgrades when we want to
// fetch the most up to date helm charts. Helm charts are stored in a tarball file
// in the default location.
var helmChartsCommand = &cli.Command{
	Name:      "helmcharts",
	Usage:     "Pull Helm chart artifacts for an airgap installation",
	UsageText: `embedded-cluster-operator pull helmcharts <installation-name>`,
	Before: func(c *cli.Context) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("pull helmcharts command must be run as root")
		}
		if len(c.Args().Slice()) != 1 {
			return fmt.Errorf("expected installation name as argument")
		}
		return nil
	},
	Action: func(c *cli.Context) error {
		in, err := fetchAndValidateInstallation(c.Context, c.Args().First())
		if err != nil {
			return err
		}

		from := in.Spec.Artifacts.HelmCharts
		logrus.Infof("fetching helm charts artifact from %s", from)
		location, err := pullArtifact(c.Context, from)
		if err != nil {
			return fmt.Errorf("unable to fetch artifact: %w", err)
		}
		defer func() {
			logrus.Infof("removing temporary directory %s", location)
			os.RemoveAll(location)
		}()

		dst := defaults.EmbeddedClusterChartsSubDir()
		src := filepath.Join(location, HelmChartsArtifactName)
		logrus.Infof("uncompressing %s", src)
		if err := tgzutils.Decompress(src, dst); err != nil {
			return fmt.Errorf("unable to uncompress helm charts: %w", err)
		}

		logrus.Infof("helm charts materialized under %s", dst)
		return nil
	},
}

// binariesCommands pulls the binary artifact from the registry running in the cluster and stores
// it locally. This command is used during cluster upgrades when we want to fetch the most up to
// date binaries. The binaries is stored in the /usr/local/bin directory and they overwrite the
// existing binaries.
var binariesCommand = &cli.Command{
	Name:      "binaries",
	Usage:     "Pull binaries artifacts for an airgap installation",
	UsageText: `embedded-cluster-operator pull binaries <installation-name>`,
	Before: func(c *cli.Context) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("pull binaries command must be run as root")
		}
		if len(c.Args().Slice()) != 1 {
			return fmt.Errorf("expected installation name as argument")
		}
		return nil
	},
	Action: func(c *cli.Context) error {
		in, err := fetchAndValidateInstallation(c.Context, c.Args().First())
		if err != nil {
			return err
		}

		from := in.Spec.Artifacts.EmbeddedClusterBinary
		logrus.Infof("fetching embedded cluster binary artifact from %s", from)
		location, err := pullArtifact(c.Context, from)
		if err != nil {
			return fmt.Errorf("unable to fetch artifact: %w", err)
		}
		defer func() {
			logrus.Infof("removing temporary directory %s", location)
			os.RemoveAll(location)
		}()
		bin := filepath.Join(location, EmbeddedClusterBinaryArtifactName)
		namedBin := filepath.Join(location, in.Spec.BinaryName)
		if err := os.Rename(bin, namedBin); err != nil {
			return fmt.Errorf("unable to rename binary: %w", err)
		}

		if err := os.Chmod(namedBin, 0755); err != nil {
			return fmt.Errorf("unable to change permissions on %s: %w", bin, err)
		}

		out := bytes.NewBuffer(nil)
		cmd := exec.Command(namedBin, "materialize")
		cmd.Stdout = out
		cmd.Stderr = out

		logrus.Infof("running command: %s materialize", bin)
		if err := cmd.Run(); err != nil {
			logrus.Error(out.String())
			return err
		}

		logrus.Infof("embedded cluster binaries materialized")

		return nil
	},
}

// fetchAndValidateInstallation fetches an Installation object from its name or directly decodes it
// and checks if it is valid for an airgap cluster deployment.
func fetchAndValidateInstallation(ctx context.Context, iname string) (*v1beta1.Installation, error) {
	in, err := decodeInstallation(ctx, iname)
	if err != nil {
		in, err = fetchInstallationFromCluster(ctx, iname)
		if err != nil {
			return nil, err
		}
	}

	if !in.Spec.AirGap {
		return nil, fmt.Errorf("installation is not airgapped")
	} else if in.Spec.Artifacts == nil {
		return nil, fmt.Errorf("installation has no artifacts")
	}

	return in, nil
}

// fetchInstallationFromCluster fetches an Installation object from the cluster.
func fetchInstallationFromCluster(ctx context.Context, iname string) (*v1beta1.Installation, error) {
	logrus.Infof("reading installation from cluster %q", iname)

	nsn := types.NamespacedName{Name: iname}
	var in v1beta1.Installation
	if err := kubecli.Get(ctx, nsn, &in); err != nil {
		return nil, fmt.Errorf("unable to get installation: %w", err)
	}

	return &in, nil
}

// decodeInstallation decodes an Installation object from a string.
func decodeInstallation(ctx context.Context, data string) (*v1beta1.Installation, error) {
	logrus.Info("decoding installation")

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	scheme := runtime.NewScheme()
	err = v1beta1.AddToScheme(scheme)
	if err != nil {
		return nil, fmt.Errorf("add to scheme: %w", err)
	}

	decode := serializer.NewCodecFactory(scheme).UniversalDeserializer().Decode
	obj, _, err := decode(decoded, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	in, ok := obj.(*v1beta1.Installation)
	if !ok {
		return nil, fmt.Errorf("unexpected object type: %T", obj)
	}

	return in, nil
}
