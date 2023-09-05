package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/helmvm/pkg/defaults"
	"github.com/replicatedhq/helmvm/pkg/goods"
	"github.com/replicatedhq/helmvm/pkg/hembed"
	pb "github.com/replicatedhq/helmvm/pkg/progressbar"
)

func pullImage(ctx context.Context, imgurl string) error {
	imgref, err := name.ParseReference(imgurl)
	if err != nil {
		return fmt.Errorf("unable to parse image reference: %w", err)
	}
	rmt, err := remote.Get(imgref)
	if err != nil {
		return fmt.Errorf("unable to get %q: %w", imgref, err)
	}
	img, err := rmt.Image()
	if err != nil {
		return err
	}
	fname := defaults.FileNameForImage(imgurl)
	outpath := fmt.Sprintf("bundle/%s", fname)
	if err := crane.Save(img, imgurl, outpath); err != nil {
		return fmt.Errorf("unable to pull image: %w", err)
	}
	return nil
}

var bundleCommand = &cli.Command{
	Name:  "build-bundle",
	Usage: "Builds the disconnected installation bundle",
	Action: func(c *cli.Context) error {
		logrus.Info("Assembling image bundle.")
		if err := os.MkdirAll("bundle", 0755); err != nil {
			return fmt.Errorf("unable to create bundle directory: %w", err)
		}
		bpath := "bundle/base_images.tar"
		dst, err := os.OpenFile(bpath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return fmt.Errorf("unable to open bundle file: %w", err)
		}
		defer dst.Close()
		loading := pb.Start(nil)
		loading.Infof("Downloading base images bundle.")
		src, err := goods.DownloadImagesBundle(defaults.K0sVersion)
		if err != nil {
			loading.Close()
			return fmt.Errorf("unable to download bundle: %w", err)
		}
		if _, err := io.Copy(dst, src); err != nil {
			loading.Close()
			return fmt.Errorf("unable to copy bundle: %w", err)
		}
		loading.Close()
		images, err := goods.ListImages()
		if err != nil {
			return fmt.Errorf("unable to list images: %w", err)
		}
		embed, err := hembed.ReadEmbedImages()
		if err != nil {
			return fmt.Errorf("unable to read embed images: %w", err)
		}
		images = append(images, embed...)
		for _, img := range images {
			loading = pb.Start(nil)
			loading.Infof(fmt.Sprintf("Pulling image %s", img))
			if err := pullImage(c.Context, img); err != nil {
				loading.Close()
				return fmt.Errorf("unable to pull image %s: %w", img, err)
			}
			loading.Close()
		}
		logrus.Info("Bundle stored under ./bundle directory.")
		return nil
	},
}
