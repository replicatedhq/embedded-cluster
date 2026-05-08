package main

import (
	"context"

	"dagger/embedded-cluster/internal/dagger"

	"go.yaml.in/yaml/v3"
)

func parseLicense(ctx context.Context, licenseFile *dagger.File) (contents string, licenseID string, channelID string, err error) {
	contents, err = licenseFile.Contents(ctx)
	if err != nil {
		return
	}
	var license struct {
		Spec struct {
			LicenseID string `yaml:"licenseID"`
			ChannelID string `yaml:"channelID"`
		} `yaml:"spec"`
	}
	if err = yaml.Unmarshal([]byte(contents), &license); err != nil {
		return
	}
	licenseID = license.Spec.LicenseID
	channelID = license.Spec.ChannelID
	return
}
