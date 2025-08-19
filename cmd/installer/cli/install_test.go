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
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts/plain"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
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

			flags := &InstallCmdFlags{
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

			err = maybePromptForAppUpdate(context.Background(), prompt, license, false)
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

func Test_preRunInstall_SkipHostPreflightsEnvVar(t *testing.T) {
	tests := []struct {
		name                   string
		envVarValue            string
		flagValue              *bool // nil means not set, true/false means explicitly set
		expectedSkipPreflights bool
	}{
		{
			name:                   "env var set to 1, no flag",
			envVarValue:            "1",
			flagValue:              nil,
			expectedSkipPreflights: true,
		},
		{
			name:                   "env var set to true, no flag",
			envVarValue:            "true",
			flagValue:              nil,
			expectedSkipPreflights: true,
		},
		{
			name:                   "env var set, flag explicitly false (flag takes precedence)",
			envVarValue:            "1",
			flagValue:              boolPtr(false),
			expectedSkipPreflights: false,
		},
		{
			name:                   "env var set, flag explicitly true",
			envVarValue:            "1",
			flagValue:              boolPtr(true),
			expectedSkipPreflights: true,
		},
		{
			name:                   "env var not set, no flag",
			envVarValue:            "",
			flagValue:              nil,
			expectedSkipPreflights: false,
		},
		{
			name:                   "env var not set, flag explicitly false",
			envVarValue:            "",
			flagValue:              boolPtr(false),
			expectedSkipPreflights: false,
		},
		{
			name:                   "env var not set, flag explicitly true",
			envVarValue:            "",
			flagValue:              boolPtr(true),
			expectedSkipPreflights: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variable
			if tt.envVarValue != "" {
				t.Setenv("SKIP_HOST_PREFLIGHTS", tt.envVarValue)
			}

			// Create a mock cobra command to simulate flag behavior
			cmd := &cobra.Command{}
			flags := &InstallCmdFlags{}

			// Add the flag to the command (similar to addInstallFlags)
			cmd.Flags().BoolVar(&flags.skipHostPreflights, "skip-host-preflights", false, "Skip host preflight checks")

			// Set the flag if explicitly provided in test
			if tt.flagValue != nil {
				err := cmd.Flags().Set("skip-host-preflights", fmt.Sprintf("%t", *tt.flagValue))
				require.NoError(t, err)
			}

			// Create a minimal runtime config for the test
			rc := runtimeconfig.New(nil)

			// Call preRunInstall (this would normally require root, but we're just testing the flag logic)
			// We expect this to fail due to non-root execution, but we can check the flag value before it fails
			err := preRunInstallLinux(cmd, flags, rc)

			// The function will fail due to non-root check, but we can verify the flag was set correctly
			// by checking the flag value before the root check fails
			assert.Equal(t, tt.expectedSkipPreflights, flags.skipHostPreflights)

			// We expect an error due to non-root execution
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "install command must be run as root")
		})
	}
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}

func Test_preRunInstallCommon_TLSValidation(t *testing.T) {
	tests := []struct {
		name        string
		tlsCertFile string
		tlsKeyFile  string
		wantErr     string
	}{
		{
			name:        "no TLS flags provided",
			tlsCertFile: "",
			tlsKeyFile:  "",
			wantErr:     "",
		},
		{
			name:        "only cert file provided",
			tlsCertFile: "cert.pem",
			tlsKeyFile:  "",
			wantErr:     "both --tls-cert and --tls-key must be provided when using custom TLS certificates",
		},
		{
			name:        "only key file provided",
			tlsCertFile: "",
			tlsKeyFile:  "key.pem",
			wantErr:     "both --tls-cert and --tls-key must be provided when using custom TLS certificates",
		},
		{
			name:        "both cert and key files provided but files don't exist",
			tlsCertFile: "nonexistent-cert.pem",
			tlsKeyFile:  "nonexistent-key.pem",
			wantErr:     "unable to read tls cert file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			cmd := &cobra.Command{}
			flags := &InstallCmdFlags{
				enableManagerExperience: false, // Non-V3 install
				tlsCertFile:             tt.tlsCertFile,
				tlsKeyFile:              tt.tlsKeyFile,
			}

			// Set up the command with proxy flags (required by preRunInstallCommon)
			mustAddProxyFlags(cmd.Flags())

			// Create a minimal runtime config for the test
			rc := runtimeconfig.New(nil)
			ki := &mockKubernetesInstallation{}

			err := preRunInstallCommon(cmd, flags, rc, ki)

			if tt.wantErr != "" {
				req.Error(err)
				req.Contains(err.Error(), tt.wantErr)
			} else {
				req.NoError(err)
			}
		})
	}
}

// Mock implementation for testing
type mockKubernetesInstallation struct{}

func (m *mockKubernetesInstallation) SetAdminConsolePort(port int)            {}
func (m *mockKubernetesInstallation) SetManagerPort(port int)                 {}
func (m *mockKubernetesInstallation) SetProxySpec(proxy *ecv1beta1.ProxySpec) {}
func (m *mockKubernetesInstallation) AdminConsolePort() int                   { return 8800 }
func (m *mockKubernetesInstallation) ManagerPort() int                        { return 30000 }
