package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// PullBinariesCmd pulls the binary artifact and stores it locally. This command is used during
// joins as well as cluster upgrades when we want to fetch the most up to date binaries. The
// binaries are stored in the /usr/local/bin directory and they overwrite the existing binaries.
//
// When using --license-id flag along with --app-slug and --channel-slug, the command will
// fetch the binary directly from the Replicated app endpoint (online mode).
// Without these flags, it will fetch the binary from the artifact path in the installation
// spec (airgap mode).
func PullBinariesCmd(cli *CLI) *cobra.Command {
	var licenseID string
	var appSlug string
	var channelSlug string
	var appVersion string

	cmd := &cobra.Command{
		Use:   "binaries INSTALLATION",
		Short: "Pull binaries artifacts for online or airgap installations",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// If license-id is set, app-slug, channel-slug, and app-version are required
			if licenseID != "" {
				if appSlug == "" {
					return fmt.Errorf("--app-slug is required when --license-id is set")
				}
				if channelSlug == "" {
					return fmt.Errorf("--channel-slug is required when --license-id is set")
				}
				if appVersion == "" {
					return fmt.Errorf("--app-version is required when --license-id is set")
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			kcli, err := cli.KCLIGetter()
			if err != nil {
				return fmt.Errorf("unable to create kube client: %w", err)
			}

			isAirgap := licenseID == ""
			in, err := fetchAndValidateInstallation(ctx, kcli, args[0], isAirgap)
			if err != nil {
				return err
			}

			var location string
			if !isAirgap {
				// For online, fetch from Replicated app using license ID
				u := releaseURL(in.Spec.MetricsBaseURL, appSlug, channelSlug, appVersion)
				logrus.Infof("fetching embedded cluster binary from %s using license ID", u)

				location, err = fetchBinaryWithLicense(u, licenseID, in.Spec.BinaryName)
				if err != nil {
					return fmt.Errorf("unable to fetch binary from Replicated app: %w", err)
				}
				logrus.Infof("successfully downloaded binary from Replicated app")
			} else {
				// For airgap, fetch from artifact path in installation spec
				from := in.Spec.Artifacts.EmbeddedClusterBinary
				logrus.Infof("fetching embedded cluster binary artifact from %s", from)

				location, err = cli.PullArtifact(ctx, kcli, from)
				if err != nil {
					return fmt.Errorf("unable to fetch artifact: %w", err)
				}
				logrus.Infof("successfully downloaded binary from artifact path")
			}

			defer func() {
				logrus.Infof("removing temporary directory %s", location)
				_ = os.RemoveAll(location)
			}()

			bin := filepath.Join(location, EmbeddedClusterBinaryArtifactName)

			// Verify the binary file exists and has content
			binInfo, err := os.Stat(bin)
			if err != nil {
				return fmt.Errorf("binary file verification failed: %w", err)
			}

			if binInfo.Size() == 0 {
				return fmt.Errorf("binary file is empty")
			}

			logrus.Infof("binary file size: %d bytes", binInfo.Size())

			namedBin := filepath.Join(location, in.Spec.BinaryName)
			if err := os.Rename(bin, namedBin); err != nil {
				return fmt.Errorf("unable to rename binary: %w", err)
			}

			if err := os.Chmod(namedBin, 0755); err != nil {
				return fmt.Errorf("unable to change permissions on %s: %w", bin, err)
			}

			materializeCmdArgs := []string{"materialize", "--data-dir", runtimeconfig.EmbeddedClusterHomeDirectory()}
			materializeCmd := exec.Command(namedBin, materializeCmdArgs...)

			logrus.Infof("running command: %s with args: %v", namedBin, materializeCmdArgs)
			if out, err := materializeCmd.CombinedOutput(); err != nil {
				logrus.Errorf("error running command:\n%s", out)
				return err
			}

			logrus.Infof("embedded cluster binaries materialized")

			return nil
		},
	}

	cmd.Flags().StringVar(&licenseID, "license-id", "", "License ID to use for fetching binary from Replicated app")
	cmd.Flags().StringVar(&appSlug, "app-slug", "", "Application slug (required when using --license-id)")
	cmd.Flags().StringVar(&channelSlug, "channel-slug", "", "Channel slug (required when using --license-id)")
	cmd.Flags().StringVar(&appVersion, "app-version", "", "Application version (required when using --license-id)")

	return cmd
}

// fetchBinaryWithLicense downloads the binary from the Replicated app using basic auth with license ID
func fetchBinaryWithLicense(url, licenseID, binaryName string) (tmpdir string, err error) {
	defer func() {
		if err != nil && tmpdir != "" {
			_ = os.RemoveAll(tmpdir)
		}
	}()

	// Create a temporary directory to store the binary
	tmpdir, err = os.MkdirTemp("", "lam-artifact-*")
	if err != nil {
		return tmpdir, fmt.Errorf("create temp dir: %w", err)
	}

	logrus.Debugf("Created temporary directory %s for binary download", tmpdir)

	// Create HTTP request with basic auth
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return tmpdir, fmt.Errorf("create request: %w", err)
	}

	logrus.Debugf("Requesting release tarball from %s using license ID auth", url)

	// Set basic auth with license ID
	req.SetBasicAuth(licenseID, "")

	// Make the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return tmpdir, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read response body for error details
		body, readErr := io.ReadAll(resp.Body)
		if readErr == nil && len(body) > 0 {
			return tmpdir, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		return tmpdir, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	logrus.Debugf("Successfully received tarball, extracting contents")

	// Stream extraction directly from response body
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return tmpdir, fmt.Errorf("create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			return tmpdir, fmt.Errorf("could not find binary file (%s) in extracted contents", binaryName)
		} else if err != nil {
			return tmpdir, err
		}

		// Skip non-regular files or files that don't match the expected binary name
		if header.Typeflag != tar.TypeReg || header.Name != binaryName {
			logrus.Debugf("Skipping file %s of type %d", header.Name, header.Typeflag)
			continue
		}

		logrus.Infof("Found binary %s", header.Name)

		// Copy the binary to the expected location
		destPath := filepath.Join(tmpdir, EmbeddedClusterBinaryArtifactName)

		outFile, err := os.Create(destPath)
		if err != nil {
			return tmpdir, fmt.Errorf("create output file: %w", err)
		}
		defer outFile.Close()

		if _, err := io.Copy(outFile, tr); err != nil {
			return tmpdir, fmt.Errorf("stream binary to file: %w", err)
		}

		logrus.Debugf("Successfully extracted binary to %s", destPath)
		return tmpdir, nil
	}
}

func releaseURL(metricsBaseURL, appSlug, channelSlug, appVersion string) string {
	return fmt.Sprintf("%s/embedded/%s/%s/%s", metricsBaseURL, appSlug, channelSlug, appVersion)
}
