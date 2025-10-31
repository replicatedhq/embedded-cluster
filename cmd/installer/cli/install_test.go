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
		assumeYes             bool
		answerYes             bool
		wantPrompt            bool
		wantErr               bool
		isErrNothingElseToAdd bool
	}{
		{
			name:           "no channel release",
			channelRelease: nil,
			wantPrompt:     false,
			wantErr:        false,
		},
		{
			name: "no license",
			channelRelease: &release.ChannelRelease{
				ChannelID:    "test-channel",
				ChannelSlug:  "test",
				AppSlug:      "app-slug",
				VersionLabel: "v1.0.0",
			},
			wantPrompt: false,
			wantErr:    true, // will fail during the test because license is required
		},
		{
			name: "version matches current release",
			channelRelease: &release.ChannelRelease{
				ChannelID:    "test-channel",
				ChannelSlug:  "test",
				AppSlug:      "app-slug",
				VersionLabel: "v1.0.0",
			},
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				response := `{"channelReleases":[{"channelId":"test-channel","versionLabel":"v1.0.0"}]}`
				w.Write([]byte(response))
			},
			wantPrompt: false,
			wantErr:    false,
		},
		{
			name: "newer version available, assumeYes true",
			channelRelease: &release.ChannelRelease{
				ChannelID:    "test-channel",
				ChannelSlug:  "test",
				AppSlug:      "app-slug",
				VersionLabel: "v1.0.0",
			},
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				response := `{"channelReleases":[{"channelId":"test-channel","versionLabel":"v2.0.0"}]}`
				w.Write([]byte(response))
			},
			assumeYes:  true,
			wantPrompt: false,
			wantErr:    false,
		},
		{
			name: "newer version available, user confirms",
			channelRelease: &release.ChannelRelease{
				ChannelID:    "test-channel",
				ChannelSlug:  "test",
				AppSlug:      "app-slug",
				VersionLabel: "v1.0.0",
			},
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				response := `{"channelReleases":[{"channelId":"test-channel","versionLabel":"v2.0.0"}]}`
				w.Write([]byte(response))
			},
			answerYes:  true,
			wantPrompt: true,
			wantErr:    false,
		},
		{
			name: "newer version available, user declines",
			channelRelease: &release.ChannelRelease{
				ChannelID:    "test-channel",
				ChannelSlug:  "test",
				AppSlug:      "app-slug",
				VersionLabel: "v1.0.0",
			},
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				response := `{"channelReleases":[{"channelId":"test-channel","versionLabel":"v2.0.0"}]}`
				w.Write([]byte(response))
			},
			answerYes:             false,
			wantPrompt:            true,
			wantErr:               true,
			isErrNothingElseToAdd: true,
		},
		{
			name: "API returns 404",
			channelRelease: &release.ChannelRelease{
				ChannelID:    "test-channel",
				ChannelSlug:  "test",
				AppSlug:      "app-slug",
				VersionLabel: "v1.0.0",
			},
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantPrompt: false,
			wantErr:    true,
		},
		{
			name: "API returns empty releases",
			channelRelease: &release.ChannelRelease{
				ChannelID:    "test-channel",
				ChannelSlug:  "test",
				AppSlug:      "app-slug",
				VersionLabel: "v1.0.0",
			},
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				response := `{"channelReleases":[]}`
				w.Write([]byte(response))
			},
			wantPrompt: false,
			wantErr:    true,
		},
		{
			name: "API returns invalid JSON",
			channelRelease: &release.ChannelRelease{
				ChannelID:    "test-channel",
				ChannelSlug:  "test",
				AppSlug:      "app-slug",
				VersionLabel: "v1.0.0",
			},
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`invalid json`))
			},
			wantPrompt: false,
			wantErr:    true,
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
			if tt.answerYes {
				in = bytes.NewBuffer([]byte("y\n"))
			} else {
				in = bytes.NewBuffer([]byte("n\n"))
			}
			out := bytes.NewBuffer([]byte{})
			prompt := plain.New(plain.WithIn(in), plain.WithOut(out))

			prompts.SetTerminal(true)
			t.Cleanup(func() { prompts.SetTerminal(false) })

			err = maybePromptForAppUpdate(context.Background(), prompt, license, tt.assumeYes)
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
