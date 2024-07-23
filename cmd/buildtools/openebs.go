package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var openebsImageComponents = map[string]string{
	"docker.io/bitnami/kubectl":             "openebs-kubectl",
	"docker.io/openebs/linux-utils":         "openebs-linux-utils",
	"docker.io/openebs/provisioner-localpv": "openebs-provisioner-localpv",
}

var openebsComponents = map[string]addonComponent{
	"openebs-provisioner-localpv": {
		getWolfiPackageName: func(k0sVersion *semver.Version, upstreamVersion string) string {
			// package name is not the same as the component name
			return "dynamic-localpv-provisioner"
		},
		upstreamVersionFlagOverride: "openebs-version",
	},
	"openebs-linux-utils": {
		upstreamVersionFlagOverride: "openebs-version",
	},
	"openebs-kubectl": {
		getWolfiPackageName: func(k0sVersion *semver.Version, upstreamVersion string) string {
			return fmt.Sprintf("kubectl-%d.%d-default", k0sVersion.Major(), k0sVersion.Minor())
		},
		getWolfiPackageVersionComparison: func(k0sVersion *semver.Version, upstreamVersion string) string {
			// match the greatest patch version of the same minor version
			return fmt.Sprintf(">=%d.%d, <%d.%d", k0sVersion.Major(), k0sVersion.Minor(), k0sVersion.Major(), k0sVersion.Minor()+1)
		},
		upstreamVersionFlagOverride: "kubectl-version",
	},
}

var updateOpenEBSAddonCommand = &cli.Command{
	Name:      "openebs",
	Usage:     "Updates the OpenEBS addon",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating openebs addon")

		rawver, err := GetMakefileVariable("K0S_VERSION")
		if err != nil {
			return fmt.Errorf("failed to get k0s version: %w", err)
		}
		k0sVersion := semver.MustParse(rawver)

		logrus.Infof("fetching wolfi apk index")
		wolfiAPKIndex, err := GetWolfiAPKIndex()
		if err != nil {
			return fmt.Errorf("failed to get APK index: %w", err)
		}

		logrus.Infof("fetching the latest openebs chart version")
		latest, err := LatestChartVersion("openebs", "openebs")
		if err != nil {
			return fmt.Errorf("failed to get the latest openebs chart version: %v", err)
		}
		latest = strings.TrimPrefix(latest, "v")
		logrus.Printf("latest openebs chart version: %s", latest)

		current := openebs.Metadata
		if current.Version == latest && !c.Bool("force") {
			logrus.Infof("openebs chart version is already up-to-date")
		} else {
			logrus.Infof("mirroring openebs chart version %s", latest)
			if err := MirrorChart("openebs", "openebs", latest); err != nil {
				return fmt.Errorf("failed to mirror openebs chart: %v", err)
			}
		}

		upstream := fmt.Sprintf("%s/openebs", os.Getenv("DESTINATION"))
		newmeta := release.AddonMetadata{
			Version:  latest,
			Location: fmt.Sprintf("oci://proxy.replicated.com/anonymous/%s", upstream),
			Images:   make(map[string]string),
		}

		values, err := release.GetValuesWithOriginalImages("openebs")
		if err != nil {
			return fmt.Errorf("failed to get openebs values: %v", err)
		}

		logrus.Infof("extracting images from chart")
		withproto := fmt.Sprintf("oci://%s", upstream)
		images, err := GetImagesFromOCIChart(withproto, "openebs", latest, values)
		if err != nil {
			return fmt.Errorf("failed to get images from admin console chart: %w", err)
		}

		// make sure we include the linux-utils image.
		images = append(images, fmt.Sprintf("docker.io/openebs/linux-utils:%s", latest))

		logrus.Infof("updating openebs images")

		if err := ApkoLogin(); err != nil {
			return fmt.Errorf("failed to apko login: %w", err)
		}

		for _, image := range images {
			logrus.Infof("updating image %s", image)

			upstreamVersion := TagFromImage(image)
			image = RemoveTagFromImage(image)

			componentName, ok := openebsImageComponents[image]
			if !ok {
				logrus.Warnf("no component found for image %s", image)
				continue
			}

			component, ok := openebsComponents[componentName]
			if !ok {
				return fmt.Errorf("no component found for component name %s", componentName)
			}

			packageName, packageVersion, err := component.getPackageNameAndVersion(wolfiAPKIndex, k0sVersion, upstreamVersion)
			if err != nil {
				return fmt.Errorf("failed to get package name and version for %s: %w", componentName, err)
			}

			logrus.Infof("building and publishing %s, %s=%s", componentName, packageName, packageVersion)

			if err := ApkoBuildAndPublish(componentName, packageName, packageVersion); err != nil {
				return fmt.Errorf("failed to apko build and publish for %s: %w", componentName, err)
			}

			digest, err := GetDigestFromBuildFile()
			if err != nil {
				return fmt.Errorf("failed to get digest from build file: %w", err)
			}

			newmeta.Images[componentName] = fmt.Sprintf("%s@%s", packageVersion, digest)
		}

		logrus.Infof("saving addon manifest")
		newmeta.ReplaceImages = true
		if err := newmeta.Save("openebs"); err != nil {
			return fmt.Errorf("failed to save metadata: %w", err)
		}

		logrus.Infof("successfully updated openebs addon")
		return nil
	},
}

var updateOpenEBSImagesCommand = &cli.Command{
	Name:      "openebs",
	Usage:     "Updates the openebs images",
	UsageText: environmentUsageText,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "openebs-version",
			Usage: "The version of openebs to use to determine image versions",
		},
		&cli.StringFlag{
			Name:  "kubectl-version",
			Usage: "The version of kubectl to use to determine image versions",
		},
	},
	Action: func(c *cli.Context) error {
		logrus.Infof("updating openebs images")

		rawver, err := GetMakefileVariable("K0S_VERSION")
		if err != nil {
			return fmt.Errorf("failed to get k0s version: %w", err)
		}
		k0sVersion := semver.MustParse(rawver)

		logrus.Infof("fetching wolfi apk index")
		wolfiAPKIndex, err := GetWolfiAPKIndex()
		if err != nil {
			return fmt.Errorf("failed to get APK index: %w", err)
		}

		current := openebs.Metadata
		newmeta := release.AddonMetadata{
			Version:  current.Version,
			Location: current.Location,
			Images:   make(map[string]string),
		}

		values, err := release.GetValuesWithOriginalImages("openebs")
		if err != nil {
			return fmt.Errorf("failed to get openebs values: %v", err)
		}

		logrus.Infof("extracting images from chart")
		images, err := GetImagesFromOCIChart(current.Location, "openebs", current.Version, values)
		if err != nil {
			return fmt.Errorf("failed to get images from admin console chart: %w", err)
		}

		// make sure we include the linux-utils image.
		images = append(images, fmt.Sprintf("docker.io/openebs/linux-utils:%s", current.Version))

		if err := ApkoLogin(); err != nil {
			return fmt.Errorf("failed to apko login: %w", err)
		}

		for _, image := range images {
			logrus.Infof("updating image %s", image)

			upstreamVersion := TagFromImage(image)
			image = RemoveTagFromImage(image)

			componentName, ok := openebsImageComponents[image]
			if !ok {
				logrus.Warnf("no component found for image %s", image)
				continue
			}

			component, ok := openebsComponents[componentName]
			if !ok {
				return fmt.Errorf("no component found for component name %s", componentName)
			}

			packageName, packageVersion, err := component.getPackageNameAndVersion(wolfiAPKIndex, k0sVersion, upstreamVersion)
			if err != nil {
				return fmt.Errorf("failed to get package name and version for %s: %w", componentName, err)
			}

			logrus.Infof("building and publishing %s, %s=%s", componentName, packageName, packageVersion)

			if err := ApkoBuildAndPublish(componentName, packageName, packageVersion); err != nil {
				return fmt.Errorf("failed to apko build and publish for %s: %w", componentName, err)
			}

			digest, err := GetDigestFromBuildFile()
			if err != nil {
				return fmt.Errorf("failed to get digest from build file: %w", err)
			}

			newmeta.Images[componentName] = fmt.Sprintf("%s@%s", packageVersion, digest)
		}

		logrus.Infof("saving addon manifest")
		newmeta.ReplaceImages = true
		if err := newmeta.Save("openebs"); err != nil {
			return fmt.Errorf("failed to save metadata: %w", err)
		}

		logrus.Infof("successfully updated openebs images")
		return nil
	},
}
