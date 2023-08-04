package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/helmvm/pkg/hembed"
)

// addChartToOptions reads the helm chart pointed by path and adds it to the provided
// EmbedOptions. Path is expected to point to a tgz file.
func addChartToOptions(path string, opts *hembed.EmbedOptions) error {
	fp, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open chart file %s: %w", path, err)
	}
	defer fp.Close()
	buf := bytes.NewBuffer(nil)
	b64enc := base64.NewEncoder(base64.StdEncoding, buf)
	defer b64enc.Close()
	if _, err := io.Copy(b64enc, fp); err != nil {
		return fmt.Errorf("failed to read chart file %s: %w", path, err)
	}
	opts.Charts = append(opts.Charts, hembed.HelmChart{Content: buf.String()})
	return nil
}

// addValuesToOptions reads the values file pointed by path and adds it to the provided
// EmbedOptions. Path is expected to point to a valid helm values file. Adds the value
// to the first chart that hasn't yet been assigned a values file.
func addValueToOptions(path string, opts *hembed.EmbedOptions) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to open values file %s: %w", path, err)
	}
	for i, chart := range opts.Charts {
		if len(chart.Values) != 0 {
			continue
		}
		opts.Charts[i].Values = string(content)
		return nil
	}
	return fmt.Errorf("no chart found to add values to")
}

var embedCommand = &cli.Command{
	Name:  "embed",
	Usage: "Embed helm charts into the binary",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "output",
			Usage:    "The binary path to write to",
			Required: true,
		},
		&cli.StringSliceFlag{
			Name:     "chart",
			Usage:    "The path to the helm chart (tgz) to embed",
			Required: true,
		},
		&cli.StringSliceFlag{
			Name:  "values",
			Usage: "The values file for the helm chart",
		},
		&cli.StringSliceFlag{
			Name:  "images",
			Usage: "A list of comma separated images to be added",
		},
	},
	Action: func(c *cli.Context) error {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("unable to get executable path: %w", err)
		}
		opts := hembed.EmbedOptions{OS: runtime.GOOS, Arch: runtime.GOARCH}
		for _, chart := range c.StringSlice("chart") {
			if err := addChartToOptions(chart, &opts); err != nil {
				return fmt.Errorf("unable to process chart: %w", err)
			}
		}
		for _, value := range c.StringSlice("values") {
			if err := addValueToOptions(value, &opts); err != nil {
				return fmt.Errorf("unable to process chart: %w", err)
			}
		}
		opts.Images = append(opts.Images, c.StringSlice("images")...)
		from, err := hembed.Embed(c.Context, exe, opts)
		if err != nil {
			return fmt.Errorf("unable to embed helm charts: %w", err)
		}
		defer from.Close()
		fpath := c.String("output")
		fp, err := os.OpenFile(fpath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0700)
		if err != nil {
			return fmt.Errorf("unable to create output file: %w", err)
		}
		defer fp.Close()
		if _, err := io.Copy(fp, from); err != nil {
			return fmt.Errorf("unable to write output file: %w", err)
		}
		return nil
	},
}
