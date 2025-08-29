package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/repo"
)

var metadataCommand = &cli.Command{
	Name:  "metadata",
	Usage: "Perform operations on the version-metadata.json file",
	Subcommands: []*cli.Command{
		metadataExtractHelmChartImagesCommand,
	},
}

var metadataExtractHelmChartImagesCommand = &cli.Command{
	Name:  "extract-helm-chart-images",
	Usage: "Extract images from Helm charts",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "metadata-path",
			Usage: "Path to the metadata file",
		},
	},
	Action: func(c *cli.Context) error {
		metadata, err := readMetadataFromFile(c.String("metadata-path"))
		if err != nil {
			return fmt.Errorf("failed to read metadata from file: %w", err)
		}

		repos := metadata.Configs.Repositories
		charts := metadata.Configs.Charts

		hcli, err := helm.NewClient(helm.HelmOptions{
			K8sVersion: metadata.Versions["Kubernetes"],
		})
		if err != nil {
			return fmt.Errorf("failed to create helm client: %w", err)
		}
		defer hcli.Close()

		images, err := extractImagesFromHelmExtensions(hcli, repos, charts)
		if err != nil {
			return fmt.Errorf("failed to extract images from helm extensions: %w", err)
		}
		log.Printf("Found %d images from helm extensions", len(images))

		for _, image := range images {
			fmt.Println(image)
		}

		return nil
	},
}

func readMetadataFromFile(path string) (*types.ReleaseMetadata, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var metadata types.ReleaseMetadata
	if err := json.Unmarshal(b, &metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}

	return &metadata, nil
}

func extractImagesFromHelmExtensions(hcli helm.Client, repos []k0sv1beta1.Repository, charts []embeddedclusterv1beta1.Chart) ([]string, error) {
	for _, entry := range repos {
		log.Printf("Adding helm repository %s", entry.Name)
		repo := &repo.Entry{
			Name:     entry.Name,
			URL:      entry.URL,
			Username: entry.Username,
			Password: entry.Password,
			CertFile: entry.CertFile,
			KeyFile:  entry.KeyFile,
			CAFile:   entry.CAFile,
		}
		if entry.Insecure != nil {
			repo.InsecureSkipTLSverify = *entry.Insecure
		}
		err := hcli.AddRepo(repo)
		if err != nil {
			return nil, fmt.Errorf("add helm repository %s: %w", entry.Name, err)
		}
	}

	var images []string
	for _, ext := range charts {
		log.Printf("Extracting images from chart %s", ext.Name)
		ims, err := extractImagesFromChart(hcli, ext)
		if err != nil {
			return nil, fmt.Errorf("extract images from chart %s: %w", ext.Name, err)
		}
		log.Printf("Found %d images in chart %s", len(ims), ext.Name)
		images = append(images, ims...)
	}
	images = helpers.UniqueStringSlice(images)
	sort.Strings(images)
	return images, nil
}

func extractImagesFromChart(hcli helm.Client, chart embeddedclusterv1beta1.Chart) ([]string, error) {
	values := map[string]interface{}{}
	if chart.Values != "" {
		err := yaml.Unmarshal([]byte(chart.Values), &values)
		if err != nil {
			return nil, fmt.Errorf("unmarshal values: %w", err)
		}
	}

	return helm.ExtractImagesFromChart(hcli, chart.ChartName, chart.Version, values)
}
