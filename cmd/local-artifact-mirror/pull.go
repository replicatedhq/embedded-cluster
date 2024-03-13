package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"k8s.io/apimachinery/pkg/types"

	"github.com/replicatedhq/embedded-cluster-operator/api/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/tgzutils"
)

const (
	EmbeddedClusterBinaryArtifactName = "embedded-cluster-amd64"
	ImagesArtifactName                = "images-amd64.tar"
	HelmChartsArtifactName            = "charts.tar.gz"
)

// pullCommand pulls artifacts from the registry running in the cluster and stores
// them locally. This command is used during cluster upgrades when we want to fetch
// the most up to date images, binaries and helm charts.
var pullCommand = &cli.Command{
	Name:        "pull",
	Usage:       "Pull artifacts for an disconnect installation",
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
	Usage:     "Pull helm chart artifacts for an airgap installation",
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
		if err := tgzutils.Uncompress(src, dst); err != nil {
			return fmt.Errorf("unable to uncompress images: %w", err)
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
		if err := os.Chmod(bin, 0755); err != nil {
			return fmt.Errorf("unable to change permissions on %s: %w", bin, err)
		}

		out := bytes.NewBuffer(nil)
		cmd := exec.Command(bin, "materialize")
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

// fetchAndValidateInstallation fetches an Installation object from its name and
// checks if it is valid for an airgap cluster deployment.
func fetchAndValidateInstallation(ctx context.Context, iname string) (*v1beta1.Installation, error) {
	kubeclient, err := kubeutils.KubeClient()
	if err != nil {
		return nil, fmt.Errorf("unable to create kube client: %w", err)
	}
	v1beta1.AddToScheme(kubeclient.Scheme())

	logrus.Infof("reading installation %q", iname)
	nsn := types.NamespacedName{Name: iname}
	var in v1beta1.Installation
	if err := kubeclient.Get(ctx, nsn, &in); err != nil {
		return nil, fmt.Errorf("unable to get installation: %w", err)
	}

	if !in.Spec.AirGap {
		return nil, fmt.Errorf("installation is not airgapped")
	} else if in.Spec.Artifacts == nil {
		return nil, fmt.Errorf("installation has no artifacts")
	}

	return &in, nil
}
