package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts/plain"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func Test_ensureAdminConsolePassword(t *testing.T) {

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

			flags := &installFlags{
				assumeYes:            tt.noPrompt,
				adminConsolePassword: tt.userPassword,
			}

			err := ensureAdminConsolePassword(flags)
			if tt.wantError {
				req.Error(err)
			} else {
				req.NoError(err)
				req.Equal(tt.wantPassword, flags.adminConsolePassword)
			}
		})
	}
}

func Test_maybePromptForAppUpdate(t *testing.T) {
	tests := []struct {
		name                  string
		channelRelease        *release.ChannelRelease
		apiHandler            func(http.ResponseWriter, *http.Request)
		confirm               bool
		wantPrompt            bool
		wantErr               bool
		isErrNothingElseToAdd bool
	}{
		{
			name: "current release should return false",
			channelRelease: &release.ChannelRelease{
				ChannelID:    "channel-id",
				ChannelSlug:  "channel-slug",
				AppSlug:      "app-slug",
				VersionLabel: "1.0.0",
			},
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"channelReleases": [
					{
						"channelId": "channel-id",
						"channelSequence": 1,
						"releaseSequence": 1,
						"versionLabel": "1.0.0",
						"isRequired": true,
						"createdAt": "2023-10-01T00:00:00Z",
						"releaseNotes": "release notes",
						"replicatedRegistryDomain": "replicated.app",
						"replicatedProxyDomain": "replicated.app"
					}
				]}`))
			},
			confirm:    false,
			wantPrompt: false,
			wantErr:    false,
		},
		{
			name: "newer release and confirm should return true",
			channelRelease: &release.ChannelRelease{
				ChannelID:    "channel-id",
				ChannelSlug:  "channel-slug",
				AppSlug:      "app-slug",
				VersionLabel: "1.0.0",
			},
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"channelReleases": [
					{
						"channelId": "channel-id",
						"channelSequence": 2,
						"releaseSequence": 2,
						"versionLabel": "2.0.0",
						"isRequired": true,
						"createdAt": "2023-10-01T00:00:00Z",
						"releaseNotes": "release notes",
						"replicatedRegistryDomain": "replicated.app",
						"replicatedProxyDomain": "replicated.app"
					}
				]}`))
			},
			confirm:    true,
			wantPrompt: true,
			wantErr:    false,
		},
		{
			name: "newer release and no confirm should return true and error",
			channelRelease: &release.ChannelRelease{
				ChannelID:    "channel-id",
				ChannelSlug:  "channel-slug",
				AppSlug:      "app-slug",
				VersionLabel: "1.0.0",
			},
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"channelReleases": [
					{
						"channelId": "channel-id",
						"channelSequence": 2,
						"releaseSequence": 2,
						"versionLabel": "2.0.0",
						"isRequired": true,
						"createdAt": "2023-10-01T00:00:00Z",
						"releaseNotes": "release notes",
						"replicatedRegistryDomain": "replicated.app",
						"replicatedProxyDomain": "replicated.app"
					}
				]}`))
			},
			confirm:               false,
			wantPrompt:            true,
			wantErr:               true,
			isErrNothingElseToAdd: true,
		},
		{
			name: "unexpected status code should return error",
			channelRelease: &release.ChannelRelease{
				ChannelID:    "channel-id",
				ChannelSlug:  "channel-slug",
				AppSlug:      "app-slug",
				VersionLabel: "1.0.0",
			},
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			},
			wantPrompt:            false,
			wantErr:               true,
			isErrNothingElseToAdd: false,
		},
		{
			name:                  "no release should return nil",
			channelRelease:        nil,
			apiHandler:            func(w http.ResponseWriter, r *http.Request) {},
			wantPrompt:            false,
			wantErr:               false,
			isErrNothingElseToAdd: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var license *kotsv1beta1.License
			releaseDataMap := map[string][]byte{}
			if tt.channelRelease != nil {
				handler := getReleasesHandler(t, tt.channelRelease.ChannelID, tt.apiHandler)
				ts := httptest.NewServer(handler)
				t.Cleanup(ts.Close)

				license = &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						LicenseID: "license-id",
						AppSlug:   "app-slug",
					},
				}

				embedStr := "# channel release object\nchannelID: %s\nchannelSlug: %s\nappSlug: %s\nversionLabel: %s\ndefaultDomains:\n  replicatedAppDomain: %s"
				releaseDataMap["release.yaml"] = []byte(fmt.Sprintf(
					embedStr,
					tt.channelRelease.ChannelID,
					tt.channelRelease.ChannelSlug,
					tt.channelRelease.AppSlug,
					tt.channelRelease.VersionLabel,
					ts.URL,
				))
			}

			err := release.SetReleaseDataForTests(releaseDataMap)
			require.NoError(t, err)

			t.Cleanup(func() {
				release.SetReleaseDataForTests(nil)
			})

			var in *bytes.Buffer
			if tt.confirm {
				in = bytes.NewBuffer([]byte("y\n"))
			} else {
				in = bytes.NewBuffer([]byte("n\n"))
			}
			out := bytes.NewBuffer([]byte{})
			prompt := plain.New(plain.WithIn(in), plain.WithOut(out))

			prompts.SetTerminal(true)
			t.Cleanup(func() { prompts.SetTerminal(false) })

			// Wrap the license for the new API
			wrappedLicense := licensewrapper.LicenseWrapper{V1: license}

			err = maybePromptForAppUpdate(context.Background(), prompt, wrappedLicense, false)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tt.wantPrompt {
				assert.Contains(t, out.String(), "Do you want to continue installing", "Prompt should have been printed")
			} else {
				assert.Empty(t, out.String(), "Prompt should not have been printed")
			}

			if tt.isErrNothingElseToAdd {
				assert.ErrorAs(t, err, &ErrorNothingElseToAdd{})
			} else {
				assert.NotErrorAs(t, err, &ErrorNothingElseToAdd{})
			}
		})
	}
}

func getReleasesHandler(t *testing.T, channelID string, apiHandler http.HandlerFunc) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/release/app-slug/pending" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("selectedChannelId") != channelID {
			t.Fatalf("unexpected selectedChannelId %s", r.URL.Query().Get("selectedChannelId"))
		}
		if r.URL.Query().Get("channelSequence") != "" {
			t.Fatalf("unexpected channelSequence %s", r.URL.Query().Get("channelSequence"))
		}
		if r.URL.Query().Get("isSemverSupported") != "true" {
			t.Fatalf("unexpected isSemverSupported %s", r.URL.Query().Get("isSemverSupported"))
		}

		apiHandler(w, r)
	}
}

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
			licenseContents: `apiVersion: kots.io/v1beta1
kind: License
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
			licenseContents: `apiVersion: kots.io/v1beta1
kind: License
spec:
  appSlug: embedded-cluster-smoke-test-staging-app
  channelID: "2cHXb1RCttzpR0xvnNWyaZCgDBP"
  isEmbeddedClusterDownloadEnabled: true
  `,
		},
		{
			name:       "valid multi-channel license, with release",
			useRelease: true,
			licenseContents: `apiVersion: kots.io/v1beta1
kind: License
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
			licenseContents: `apiVersion: kots.io/v1beta1
kind: License
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
			licenseContents: `apiVersion: kots.io/v1beta1
kind: License
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
			licenseContents: `apiVersion: kots.io/v1beta1
kind: License
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
			licenseContents: `apiVersion: kots.io/v1beta1
kind: License
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
			licenseContents: `apiVersion: kots.io/v1beta1
kind: License
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
			licenseContents: `apiVersion: kots.io/v1beta1
kind: License
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

			t.Cleanup(func() {
				release.SetReleaseDataForTests(nil)
			})

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

func Test_verifyProxyConfig(t *testing.T) {
	tests := []struct {
		name                  string
		proxy                 *ecv1beta1.ProxySpec
		confirm               bool
		assumeYes             bool
		wantErr               bool
		isErrNothingElseToAdd bool
	}{
		{
			name:    "no proxy set",
			proxy:   nil,
			wantErr: false,
		},
		{
			name: "http proxy set without https proxy and user confirms",
			proxy: &ecv1beta1.ProxySpec{
				HTTPProxy: "http://proxy:8080",
			},
			confirm: true,
			wantErr: false,
		},
		{
			name: "http proxy set without https proxy and user declines",
			proxy: &ecv1beta1.ProxySpec{
				HTTPProxy: "http://proxy:8080",
			},
			confirm:               false,
			wantErr:               true,
			isErrNothingElseToAdd: true,
		},
		{
			name: "http proxy set without https proxy and assumeYes is true",
			proxy: &ecv1beta1.ProxySpec{
				HTTPProxy: "http://proxy:8080",
			},
			assumeYes: true,
			wantErr:   false,
		},
		{
			name: "both proxies set",
			proxy: &ecv1beta1.ProxySpec{
				HTTPProxy:  "http://proxy:8080",
				HTTPSProxy: "https://proxy:8080",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var in *bytes.Buffer
			if tt.confirm {
				in = bytes.NewBuffer([]byte("y\n"))
			} else {
				in = bytes.NewBuffer([]byte("n\n"))
			}
			out := bytes.NewBuffer([]byte{})
			mockPrompt := plain.New(plain.WithIn(in), plain.WithOut(out))

			prompts.SetTerminal(true)
			t.Cleanup(func() { prompts.SetTerminal(false) })

			err := verifyProxyConfig(tt.proxy, mockPrompt, tt.assumeYes)
			if tt.wantErr {
				require.Error(t, err)
				if tt.isErrNothingElseToAdd {
					assert.ErrorAs(t, err, &ErrorNothingElseToAdd{})
				}
				if tt.proxy != nil && tt.proxy.HTTPProxy != "" && tt.proxy.HTTPSProxy == "" && !tt.assumeYes {
					assert.Contains(t, out.String(), "Typically --https-proxy should be set if --http-proxy is set")
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_ignoreAppPreflights_FlagVisibility(t *testing.T) {
	tests := []struct {
		name                        string
		enableV3EnvVar              string
		expectedFlagShouldBeVisible bool
	}{
		{
			name:                        "ENABLE_V3 not set - flag should be visible",
			enableV3EnvVar:              "",
			expectedFlagShouldBeVisible: true,
		},
		{
			name:                        "ENABLE_V3 set to 1 - flag should be hidden",
			enableV3EnvVar:              "1",
			expectedFlagShouldBeVisible: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean environment
			os.Unsetenv("ENABLE_V3")

			// Set environment variable if specified
			if tt.enableV3EnvVar != "" {
				t.Setenv("ENABLE_V3", tt.enableV3EnvVar)
			}

			flags := &installFlags{}
			enableV3 := isV3Enabled()
			flagSet := newLinuxInstallFlags(flags, enableV3)

			// Check if the flag exists
			flag := flagSet.Lookup("ignore-app-preflights")
			flagExists := flag != nil

			assert.Equal(t, tt.expectedFlagShouldBeVisible, flagExists, "Flag visibility should match expected")

			if flagExists {
				// Test flag properties
				assert.Equal(t, "ignore-app-preflights", flag.Name)
				assert.Equal(t, "false", flag.DefValue) // Default should be false
				assert.Equal(t, "Allow bypassing app preflight failures", flag.Usage)
				assert.Equal(t, "bool", flag.Value.Type())

				// Test flag targets - should be Linux only
				targetAnnotation := flag.Annotations[flagAnnotationTarget]
				require.NotNil(t, targetAnnotation, "Flag should have target annotation")
				assert.Contains(t, targetAnnotation, flagAnnotationTargetValueLinux)
			}
		})
	}
}

func Test_ignoreAppPreflights_FlagParsing(t *testing.T) {
	tests := []struct {
		name                     string
		args                     []string
		enableV3                 bool
		expectedIgnorePreflights bool
		expectError              bool
	}{
		{
			name:                     "flag not provided, V3 disabled",
			args:                     []string{},
			enableV3:                 false,
			expectedIgnorePreflights: false,
			expectError:              false,
		},
		{
			name:                     "flag set to true, V3 disabled",
			args:                     []string{"--ignore-app-preflights"},
			enableV3:                 false,
			expectedIgnorePreflights: true,
			expectError:              false,
		},
		{
			name:                     "flag set but V3 enabled - should error",
			args:                     []string{"--ignore-app-preflights"},
			enableV3:                 true,
			expectedIgnorePreflights: false,
			expectError:              true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable for V3 testing
			if tt.enableV3 {
				t.Setenv("ENABLE_V3", "1")
			}

			// Create a flagset similar to how newLinuxInstallFlags works
			flags := &installFlags{}
			flagSet := newLinuxInstallFlags(flags, tt.enableV3)

			// Create a command to test flag parsing
			cmd := &cobra.Command{
				Use: "test",
				Run: func(cmd *cobra.Command, args []string) {},
			}
			cmd.Flags().AddFlagSet(flagSet)

			// Try to parse the arguments
			err := cmd.Flags().Parse(tt.args)
			if tt.expectError {
				assert.Error(t, err, "Flag parsing should fail when flag doesn't exist")
			} else {
				assert.NoError(t, err, "Flag parsing should succeed")
				// Check the flag value only if parsing succeeded
				assert.Equal(t, tt.expectedIgnorePreflights, flags.ignoreAppPreflights)
			}
		})
	}
}

func Test_k0sConfigFromFlags(t *testing.T) {
	tests := []struct {
		name                string
		podCIDR             string
		serviceCIDR         string
		globalCIDR          *string
		expectedPodCIDR     string
		expectedServiceCIDR string
		wantErr             bool
	}{
		{
			name:                "pod and service CIDRs set",
			podCIDR:             "10.0.0.0/24",
			serviceCIDR:         "10.1.0.0/24",
			globalCIDR:          nil,
			expectedPodCIDR:     "10.0.0.0/24",
			expectedServiceCIDR: "10.1.0.0/24",
			wantErr:             false,
		},
		{
			name:                "custom pod and service CIDRs",
			podCIDR:             "192.168.0.0/16",
			serviceCIDR:         "10.96.0.0/12",
			globalCIDR:          nil,
			expectedPodCIDR:     "192.168.0.0/16",
			expectedServiceCIDR: "10.96.0.0/12",
			wantErr:             false,
		},
		{
			name:                "global CIDR should not affect k0s config",
			podCIDR:             "10.0.0.0/25",
			serviceCIDR:         "10.0.0.128/25",
			globalCIDR:          stringPtr("10.0.0.0/24"),
			expectedPodCIDR:     "10.0.0.0/25",
			expectedServiceCIDR: "10.0.0.128/25",
			wantErr:             false,
		},
		{
			name:                "IPv4 CIDRs with different masks",
			podCIDR:             "172.16.0.0/20",
			serviceCIDR:         "172.17.0.0/20",
			globalCIDR:          nil,
			expectedPodCIDR:     "172.16.0.0/20",
			expectedServiceCIDR: "172.17.0.0/20",
			wantErr:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			flags := &installFlags{
				cidrConfig: &newconfig.CIDRConfig{
					PodCIDR:     tt.podCIDR,
					ServiceCIDR: tt.serviceCIDR,
					GlobalCIDR:  tt.globalCIDR,
				},
				networkInterface: "",
				overrides:        "",
			}
			installCfg := &installConfig{}

			cfg, err := k0sConfigFromFlags(flags, installCfg)

			if tt.wantErr {
				req.Error(err)
				return
			}

			req.NoError(err)
			req.NotNil(cfg)
			req.NotNil(cfg.Spec)
			req.NotNil(cfg.Spec.Network)

			// Verify pod CIDR is set correctly if expected
			if tt.expectedPodCIDR != "" {
				req.Equal(tt.expectedPodCIDR, cfg.Spec.Network.PodCIDR,
					"Pod CIDR should be set correctly in k0s config")
			}

			// Verify service CIDR is set correctly if expected
			if tt.expectedServiceCIDR != "" {
				req.Equal(tt.expectedServiceCIDR, cfg.Spec.Network.ServiceCIDR,
					"Service CIDR should be set correctly in k0s config")
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}

func Test_buildInstallDerivedConfig_TLS(t *testing.T) {
	// Create a temporary directory for test certificates
	tmpdir := t.TempDir()

	// Create valid test certificate and key files
	certPath := filepath.Join(tmpdir, "test-cert.pem")
	keyPath := filepath.Join(tmpdir, "test-key.pem")

	// Valid test certificate and key data
	certData := `-----BEGIN CERTIFICATE-----
MIIDizCCAnOgAwIBAgIUJaAILNY7l9MR4mfMP4WiUObo6TIwDQYJKoZIhvcNAQEL
BQAwVTELMAkGA1UEBhMCVVMxDTALBgNVBAgMBFRlc3QxDTALBgNVBAcMBFRlc3Qx
DTALBgNVBAoMBFRlc3QxGTAXBgNVBAMMEHRlc3QuZXhhbXBsZS5jb20wHhcNMjUw
ODE5MTcwNTU4WhcNMjYwODE5MTcwNTU4WjBVMQswCQYDVQQGEwJVUzENMAsGA1UE
CAwEVGVzdDENMAsGA1UEBwwEVGVzdDENMAsGA1UECgwEVGVzdDEZMBcGA1UEAwwQ
dGVzdC5leGFtcGxlLmNvbTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB
AMhkRyxUJE4JLrTbqq/Etdvd2osmkZJA5GXCRkWcGLBppNNqO1v8K0zy5dV9jgno
gjeQD2nTqZ++vmzR3wPObeB6MJY+2SYtFHvnT3G9HR4DcSX3uHUOBDjbUsW0OT6z
weT3t3eTVqNIY96rZRHz9VYrdC4EPlWyfoYTCHceZey3AqSgHWnHIxVaATWT/LFQ
yvRRlEBNf7/M5NX0qis91wKgGwe6u+P/ebmT1cXURufM0jSAMUbDIqr73Qq5m6t4
fv6/8XKAiVpA1VcACvR79kTi6hYMls88ShHuYLJK175ZQfkeJx77TI/UebALL9CZ
SCI1B08SMZOsr9GQMOKNIl8CAwEAAaNTMFEwHQYDVR0OBBYEFCQWAH7mJ0w4Iehv
PL72t8GCJ90uMB8GA1UdIwQYMBaAFCQWAH7mJ0w4IehvPL72t8GCJ90uMA8GA1Ud
EwEB/wQFMAMBAf8wDQYJKoZIhvcNAQELBQADggEBAFfEICcE4eFZkRfjcEkvrJ3T
KmMikNP2nPXv3h5Ie0DpprejPkDyOWe+UJBanYwAf8xXVwRTmE5PqQhEik2zTBlN
N745Izq1cUYIlyt9GHHycx384osYHKkGE9lAPEvyftlc9hCLSu/FVQ3+8CGwGm9i
cFNYLx/qrKkJxT0Lohi7VCAf7+S9UWjIiLaETGlejm6kPNLRZ0VoxIPgUmqePXfp
6gY5FSIzvH1kZ+bPZ3nqsGyT1l7TsubeTPDDGhpKgIFzcJX9WeY//bI4q1SpU1Fl
koNnBhDuuJxjiafIFCz4qVlf0kmRrz4jeXGXym8IjxUq0EpMgxGuSIkguPKiwFQ=
-----END CERTIFICATE-----`

	keyData := `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDIZEcsVCROCS60
26qvxLXb3dqLJpGSQORlwkZFnBiwaaTTajtb/CtM8uXVfY4J6II3kA9p06mfvr5s
0d8Dzm3gejCWPtkmLRR7509xvR0eA3El97h1DgQ421LFtDk+s8Hk97d3k1ajSGPe
q2UR8/VWK3QuBD5Vsn6GEwh3HmXstwKkoB1pxyMVWgE1k/yxUMr0UZRATX+/zOTV
9KorPdcCoBsHurvj/3m5k9XF1EbnzNI0gDFGwyKq+90KuZureH7+v/FygIlaQNVX
AAr0e/ZE4uoWDJbPPEoR7mCySte+WUH5Hice+0yP1HmwCy/QmUgiNQdPEjGTrK/R
kDDijSJfAgMBAAECggEAHnl1g23GWaG22yU+110cZPPfrOKwJ6Q7t6fsRODAtm9S
dB5HKa13LkwQHL/rzmDwEKAVX/wi4xrAXc8q0areddFPO0IShuY7I76hC8R9PZe7
aNE72X1IshbUhyFpxTnUBkyPt50OA2XaXj4FcE3/5NtV3zug+SpcaGpTkr3qNS24
0Qf5X8AA1STec81c4BaXc8GgLsXz/4kWUSiwK0fjXcIpHkW28gtUyVmYu3FAPSdo
4bKdbqNUiYxF+JYLCQ9PyvFAqy7EhFLM4QkMICnSBNqNCPq3hVOr8K4V9luNnAmS
oU5gEHXmGM8a+kkdvLoZn3dO5tRk8ctV0vnLMYnXrQKBgQDl4/HDbv3oMiqS9nJK
+vQ7/yzLUb00fVzvWbvSLdEfGCgbRlDRKkNMgI5/BnFTJcbG5o3rIdBW37FY3iAy
p4iIm+VGiDz4lFApAQdiQXk9d2/mfB9ZVryUsKskvk6WTjom6+BRSvakqe2jIa/i
udnMFNGkJj6HzZqss1LKDiR5DQKBgQDfJqj5AlCyNUxjokWMH0BapuBVSHYZnxxD
xR5xX/5Q5fKDBpp4hMn8vFS4L8a5mCOBUPbuxEj7KY0Ho5bqYWmt+HyxP5TvDS9h
ZqgDdJuWdLB4hfzlUKekufFrpALvUT4AbmYdQ+ufkggU0mWGCfKaijlk4Hy/VRH7
w5ConbJWGwKBgADkF0XIoldKCnwzVFISEuxAmu3WzULs0XVkBaRU5SCXuWARr7J/
1W7weJzpa3sFBHY04ovsv5/2kftkMP/BQng1EnhpgsL74Cuog1zQICYq1lYwWPbB
rU1uOduUmT1f5D3OYDowbjBJMFCXitT4H235Dq7yLv/bviO5NjLuRxnpAoGBAJBj
LnA4jEhS7kOFiuSYkAZX9c2Y3jnD1wEOuZz4VNC5iMo46phSq3Np1JN87mPGSirx
XWWvAd3py8QGmK69KykTIHN7xX1MFb07NDlQKSAYDttdLv6dymtumQRiEjgRZEHZ
LR+AhCQy1CHM5T3uj9ho2awpCO6wN7uklaRUrUDDAoGBAK/EPsIxm5yj+kFIc/qk
SGwCw13pfbshh9hyU6O//h3czLnN9dgTllfsC7qqxsgrMCVZO9ZIfh5eb44+p7Id
r3glM4yhSJwf/cAWmt1A7DGOYnV7FF2wkDJJPX/Vag1uEsqrzwnAdFBymK5dwDsu
oxhVqyhpk86rf0rT5DcD/sBw
-----END PRIVATE KEY-----`

	err := os.WriteFile(certPath, []byte(certData), 0644)
	require.NoError(t, err)
	err = os.WriteFile(keyPath, []byte(keyData), 0644)
	require.NoError(t, err)

	tests := []struct {
		name        string
		tlsCertFile string
		tlsKeyFile  string
		wantErr     string
		expectTLS   bool
	}{
		{
			name:        "no TLS files provided",
			tlsCertFile: "",
			tlsKeyFile:  "",
			wantErr:     "",
			expectTLS:   false,
		},
		{
			name:        "cert file does not exist",
			tlsCertFile: filepath.Join(tmpdir, "nonexistent.pem"),
			tlsKeyFile:  keyPath,
			wantErr:     "failed to read TLS certificate",
			expectTLS:   false,
		},
		{
			name:        "key file does not exist",
			tlsCertFile: certPath,
			tlsKeyFile:  filepath.Join(tmpdir, "nonexistent.key"),
			wantErr:     "failed to read TLS key",
			expectTLS:   false,
		},
		{
			name: "invalid cert file",
			tlsCertFile: func() string {
				invalidCertPath := filepath.Join(tmpdir, "invalid-cert.pem")
				os.WriteFile(invalidCertPath, []byte("invalid cert data"), 0644)
				return invalidCertPath
			}(),
			tlsKeyFile: keyPath,
			wantErr:    "failed to parse TLS certificate",
			expectTLS:  false,
		},
		{
			name:        "valid cert and key files",
			tlsCertFile: certPath,
			tlsKeyFile:  keyPath,
			wantErr:     "",
			expectTLS:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := &installFlags{
				tlsCertFile: tt.tlsCertFile,
				tlsKeyFile:  tt.tlsKeyFile,
			}

			installCfg, err := buildInstallConfig(flags)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)

				if tt.expectTLS {
					assert.NotEmpty(t, installCfg.tlsCertBytes, "TLS cert bytes should be populated")
					assert.NotEmpty(t, installCfg.tlsKeyBytes, "TLS key bytes should be populated")
					assert.NotNil(t, installCfg.tlsCert.Certificate, "TLS cert should be loaded")
				} else {
					assert.Empty(t, installCfg.tlsCertBytes, "TLS cert bytes should be empty")
					assert.Empty(t, installCfg.tlsKeyBytes, "TLS key bytes should be empty")
				}
			}
		})
	}
}
