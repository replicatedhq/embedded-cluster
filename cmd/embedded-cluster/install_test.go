package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/stretchr/testify/require"
)

func Test_getLicenseFromFilepath(t *testing.T) {
	tests := []struct {
		name            string
		licenseContents string
		wantErr         string
		useRelease      bool
	}{
		{
			name:    "no license, no release",
			wantErr: "",
		},
		{
			name:       "no license, with release",
			useRelease: true,
			wantErr:    `no license was provided for embedded-cluster-smoke-test-staging-app and one is required, please rerun with '--license <path to license file>'`,
		},
		{
			name: "valid license, no release",
			licenseContents: `
spec:
  appSlug: embedded-cluster-smoke-test-staging-app
  channelID: "2cHXb1RCttzpR0xvnNWyaZCgDBP"
  isEmbeddedClusterDownloadEnabled: true
  `,
			wantErr: "a license was provided but no release was found in binary, please rerun without the license flag",
		},
		{
			name:       "valid license, with release",
			useRelease: true,
			licenseContents: `
spec:
  appSlug: embedded-cluster-smoke-test-staging-app
  channelID: "2cHXb1RCttzpR0xvnNWyaZCgDBP"
  isEmbeddedClusterDownloadEnabled: true
  `,
		},
		{
			name:       "expired license, with release",
			useRelease: true,
			licenseContents: `
spec:
  appSlug: embedded-cluster-smoke-test-staging-app
  channelID: "2cHXb1RCttzpR0xvnNWyaZCgDBP"
  isEmbeddedClusterDownloadEnabled: true
  entitlements:
    expires_at:
      description: License Expiration
      signature: {}
      title: Expiration
      value: "2024-06-03T00:00:00Z"
      valueType: String
  `,
			wantErr: "license expired on 2024-06-03 00:00:00 +0000 UTC, please provide a valid license",
		},
		{
			name:       "license with no expiration, with release",
			useRelease: true,
			licenseContents: `
spec:
  appSlug: embedded-cluster-smoke-test-staging-app
  channelID: "2cHXb1RCttzpR0xvnNWyaZCgDBP"
  isEmbeddedClusterDownloadEnabled: true
  entitlements:
    expires_at:
      description: License Expiration
      signature: {}
      title: Expiration
      value: ""
      valueType: String
  `,
		},
		{
			name:       "license with 100 year expiration, with release",
			useRelease: true,
			licenseContents: `
spec:
  appSlug: embedded-cluster-smoke-test-staging-app
  channelID: "2cHXb1RCttzpR0xvnNWyaZCgDBP"
  isEmbeddedClusterDownloadEnabled: true
  entitlements:
    expires_at:
      description: License Expiration
      signature: {}
      title: Expiration
      value: "2124-06-03T00:00:00Z"
      valueType: String
  `,
		},
		{
			name:       "embedded cluster not enabled, with release",
			useRelease: true,
			licenseContents: `
spec:
  appSlug: embedded-cluster-smoke-test-staging-app
  channelID: "2cHXb1RCttzpR0xvnNWyaZCgDBP"
  isEmbeddedClusterDownloadEnabled: false
  `,
			wantErr: "license does not have embedded cluster enabled, please provide a valid license",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			tmpdir, err := os.MkdirTemp("", "license")
			defer os.RemoveAll(tmpdir)
			req.NoError(err)

			licenseFile, err := os.Create(tmpdir + "/license.yaml")
			req.NoError(err)
			_, err = licenseFile.Write([]byte(tt.licenseContents))
			req.NoError(err)

			dataMap := map[string][]byte{}
			if tt.useRelease {
				dataMap["release.yaml"] = []byte(`
# channel release object
channelID: "2cHXb1RCttzpR0xvnNWyaZCgDBP"
channelSlug: "CI"
appSlug: "embedded-cluster-smoke-test-staging-app"
versionLabel: testversion
`)
			}
			err = release.SetReleaseDataForTests(dataMap)
			req.NoError(err)

			if tt.licenseContents != "" {
				_, err = getLicenseFromFilepath(filepath.Join(tmpdir, "license.yaml"))
			} else {
				_, err = getLicenseFromFilepath("")
			}

			if tt.wantErr != "" {
				req.EqualError(err, tt.wantErr)
			} else {
				req.NoError(err)
			}
		})
	}
}
