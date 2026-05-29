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

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// PullBinariesCmd pulls the binary artifact and stores it locally. This command is used during
// joins as well as cluster upgrades when we want to fetch the most up to date binaries. The
// binaries are stored in the /usr/local/bin directory and they overwrite the existing binaries.
//
// When using --license-id flag along with --app-slug and --channel-id, the command will fetch the
// binary directly from the Replicated app endpoint (online mode). Without these flags, it will
// fetch the binary from the artifact path in the installation spec (airgap mode).
func PullBinariesCmd(cli *CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "binaries INSTALLATION",
		Short: "Pull binaries artifacts for online or airgap installations",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			cli.bindFlags(cmd.Flags())

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			kcli, err := cli.KCLIGetter()
			if err != nil {
				return fmt.Errorf("unable to create kube client: %w", err)
			}

			in, err := fetchAndValidateInstallation(ctx, kcli, args[0])
			if err != nil {
				return err
			}

			licenseID := cli.V.GetString("license-id")
			appSlug := cli.V.GetString("app-slug")
			channelID := cli.V.GetString("channel-id")
			appVersion := cli.V.GetString("app-version")

			if !in.Spec.AirGap {
				if licenseID == "" || appSlug == "" || channelID == "" || appVersion == "" {
					return fmt.Errorf("--license-id, --app-slug, --channel-id, and --app-version are required for online installations")
				}
			}

			var location string
			if !in.Spec.AirGap {
				// For online, fetch from Replicated app using license ID
				u := releaseURL(in.Spec.MetricsBaseURL, appSlug, channelID, appVersion)
				logrus.Infof("fetching embedded cluster binary from %s using license ID", u)

				location, err = fetchBinaryWithLicense(u, licenseID, appSlug)
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

			materializeCmdArgs := []string{"materialize", "--data-dir", cli.RC.EmbeddedClusterHomeDirectory()}
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

	cmd.Flags().String("license-id", "", "License ID to use for authentication with replicated.app (required for online installations)")
	cmd.Flags().String("app-slug", "", "Application slug for fetching binary from replicated.app (required for online installations)")
	cmd.Flags().String("channel-id", "", "Channel ID for fetching binary from replicated.app (required for online installations)")
	cmd.Flags().String("app-version", "", "Application version for fetching binary from replicated.app (required for online installations)")

	return cmd
}

// fetchBinaryWithLicense downloads the binary from the Replicated app using basic auth with license ID
func fetchBinaryWithLicense(url, licenseID, binaryName string) (string, error) {
	// Create a temporary directory to store the binary
	tmpdir, err := os.MkdirTemp("", "lam-artifact-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	logrus.Debugf("Created temporary directory %s for binary download", tmpdir)

	logrus.Debugf("Requesting release tarball from %s using license ID auth", url)
	body, err := doBinaryRequest(url, licenseID)
	if err != nil {
		_ = os.RemoveAll(tmpdir)
		return "", fmt.Errorf("create request: %w", err)
	}
	defer body.Close()

	logrus.Debugf("Successfully received tarball, extracting contents")

	// Stream extraction directly from response body
	gzr, err := gzip.NewReader(body)
	if err != nil {
		_ = os.RemoveAll(tmpdir)
		return "", fmt.Errorf("create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	// Copy the binary to the expected location
	destPath := filepath.Join(tmpdir, EmbeddedClusterBinaryArtifactName)

	if err := extractBinaryFromTarball(tr, binaryName, destPath); err != nil {
		_ = os.RemoveAll(tmpdir)
		return "", fmt.Errorf("extract tarball: %w", err)
	}

	return tmpdir, nil
}

func doBinaryRequest(url, licenseID string) (io.ReadCloser, error) {
	// Create HTTP request with basic auth
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set basic auth with license ID
	req.SetBasicAuth(licenseID, licenseID)

	// Make the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Read response body for error details
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr == nil && len(body) > 0 {
			return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	return resp.Body, nil
}

func extractBinaryFromTarball(tr *tar.Reader, binaryName string, destPath string) error {
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("could not find binary file (%s) in extracted contents", binaryName)
		} else if err != nil {
			return err
		}

		// Skip non-regular files or files that don't match the expected binary name
		if header.Typeflag != tar.TypeReg || header.Name != binaryName {
			logrus.Debugf("Skipping file %s of type %d", header.Name, header.Typeflag)
			continue
		}

		logrus.Infof("Found binary %s", header.Name)

		outFile, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer outFile.Close()

		if _, err := io.Copy(outFile, tr); err != nil {
			return fmt.Errorf("stream binary to file: %w", err)
		}

		logrus.Debugf("Successfully extracted binary to %s", destPath)
		return nil
	}
}

func releaseURL(metricsBaseURL, appSlug, channelID, appVersion string) string {
	return fmt.Sprintf("%s/embedded/%s/%s/%s", metricsBaseURL, appSlug, channelID, appVersion)
}
