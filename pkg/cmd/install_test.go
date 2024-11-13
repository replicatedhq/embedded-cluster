package cmd

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
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
			name:       "valid multi-channel license, with release",
			useRelease: true,
			licenseContents: `
spec:
  appSlug: embedded-cluster-smoke-test-staging-app
  channelID: "OtherChannelID"
  isEmbeddedClusterDownloadEnabled: true
  channels:
    - channelID: OtherChannelID
      channelName: OtherChannel
      channelSlug: other-channel
      isDefault: true
    - channelID: 2cHXb1RCttzpR0xvnNWyaZCgDBP
      channelName: ExpectedChannel
      channelSlug: expected-channel
      isDefault: false
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
		{
			name:       "incorrect license (multichan license)",
			useRelease: true,
			licenseContents: `
spec:
  appSlug: embedded-cluster-smoke-test-staging-app
  channelID: "2i9fCbxTNIhuAOaC6MoKMVeGzuK"
  isEmbeddedClusterDownloadEnabled: false
  channels:
    - channelID: 2i9fCbxTNIhuAOaC6MoKMVeGzuK
      channelName: Stable
      channelSlug: stable
      isDefault: true
    - channelID: 4l9fCbxTNIhuAOaC6MoKMVeV3K
      channelName: Alternate
      channelSlug: alternate
      isDefault: false
  `,
			wantErr: "binary channel 2cHXb1RCttzpR0xvnNWyaZCgDBP (CI) not present in license, channels allowed by license are: stable (2i9fCbxTNIhuAOaC6MoKMVeGzuK), alternate (4l9fCbxTNIhuAOaC6MoKMVeV3K)",
		},
		{
			name:       "incorrect license (pre-multichan license)",
			useRelease: true,
			licenseContents: `
spec:
  appSlug: embedded-cluster-smoke-test-staging-app
  channelID: "2i9fCbxTNIhuAOaC6MoKMVeGzuK"
  channelName: "Stable"
  isEmbeddedClusterDownloadEnabled: false
  `,
			wantErr: "binary channel 2cHXb1RCttzpR0xvnNWyaZCgDBP (CI) not present in license, channels allowed by license are: Stable (2i9fCbxTNIhuAOaC6MoKMVeGzuK)",
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

			var channelRelease *release.ChannelRelease
			if tt.useRelease {
				channelRelease = &release.ChannelRelease{
					ChannelID:    "2cHXb1RCttzpR0xvnNWyaZCgDBP",
					ChannelSlug:  "CI",
					AppSlug:      "embedded-cluster-smoke-test-staging-app",
					VersionLabel: "testversion",
				}
			}

			if tt.licenseContents != "" {
				_, err = getLicenseFromFilepath(filepath.Join(tmpdir, "license.yaml"), channelRelease)
			} else {
				_, err = getLicenseFromFilepath("", channelRelease)
			}

			if tt.wantErr != "" {
				req.EqualError(err, tt.wantErr)
			} else {
				req.NoError(err)
			}
		})
	}
}

func Test_validateAdminConsolePassword(t *testing.T) {

	tests := []struct {
		name          string
		password      string
		passwordCheck string
		wantSuccess   bool
	}{
		{
			name:          "passwords match, with 3 characters length",
			password:      "123",
			passwordCheck: "123",
			wantSuccess:   false,
		},
		{
			name:          "passwords don't match, with 3 characters length",
			password:      "123",
			passwordCheck: "nop",
			wantSuccess:   false,
		},
		{
			name:          "passwords don't match, with 6 characters length",
			password:      "123456",
			passwordCheck: "nmatch",
			wantSuccess:   false,
		},
		{
			name:          "passwords match, with 6 characters length",
			password:      "123456",
			passwordCheck: "123456",
			wantSuccess:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			success := validateAdminConsolePassword(tt.password, tt.passwordCheck)
			if tt.wantSuccess {
				req.True(success)
			} else {
				req.False(success)
			}
		})
	}
}

func Test_maybeAskAdminConsolePassword(t *testing.T) {

	tests := []struct {
		name         string
		userPassword string
		noPrompt     bool
		wantPassword string
		wantError    bool
	}{
		{
			name:         "no user provided password, no-prompt true",
			userPassword: "",
			noPrompt:     true,
			wantPassword: "password",
			wantError:    false,
		},
		{
			name:         "invalid user provided password, no-prompt false",
			userPassword: "123",
			noPrompt:     false,
			wantPassword: "",
			wantError:    true,
		},
		{
			name:         "user provided password, no-prompt true",
			userPassword: "123456",
			noPrompt:     true,
			wantPassword: "123456",
			wantError:    false,
		},
		{
			name:         "user provided password, no-prompt false",
			userPassword: "123456",
			noPrompt:     false,
			wantPassword: "123456",
			wantError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			flags := installCommand().Flags
			flagSet := flag.NewFlagSet("test", 0)
			for _, flag := range flags {
				flag.Apply(flagSet)
			}
			flagSet.Set("no-prompt", strconv.FormatBool(tt.noPrompt))
			flagSet.Set("admin-console-password", tt.userPassword)
			c := cli.NewContext(cli.NewApp(), flagSet, nil)
			passwordSet, err := maybeAskAdminConsolePassword(c)

			if tt.wantError {
				req.Error(err)
			} else {
				req.Equal(tt.wantPassword, passwordSet)
			}
		})
	}
}
