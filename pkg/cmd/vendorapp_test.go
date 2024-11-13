package cmd

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/pkg/prompts/plain"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func Test_getCurrentAppChannelRelease(t *testing.T) {
	type args struct {
		channelID string
	}
	tests := []struct {
		name       string
		args       args
		apiHandler func(http.ResponseWriter, *http.Request)
		want       *apiChannelRelease
		wantErr    bool
	}{
		{
			name: "should return current channel release",
			args: args{
				channelID: "channel-id",
			},
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"channelReleases": [
					{
						"channelSequence": 2,
						"releaseSequence": 2,
						"versionLabel": "2.0.0",
						"isRequired": true,
						"createdAt": "2023-10-01T00:00:00Z",
						"releaseNotes": "release notes"
					}
				]}`))
			},
			want: &apiChannelRelease{
				ChannelSequence: 2,
				ReleaseSequence: 2,
				VersionLabel:    "2.0.0",
				IsRequired:      true,
				CreatedAt:       "2023-10-01T00:00:00Z",
				ReleaseNotes:    "release notes",
			},
			wantErr: false,
		},
		{
			name: "unexpected status code should return error",
			args: args{
				channelID: "channel-id",
			},
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := getReleasesHandler(t, tt.args.channelID, tt.apiHandler)
			apiURL := startFakeServer(t, handler)
			license := &kotsv1beta1.License{
				Spec: kotsv1beta1.LicenseSpec{
					LicenseID: "license-id",
					AppSlug:   "app-slug",
					Endpoint:  apiURL.String(),
				},
			}

			got, err := getCurrentAppChannelRelease(context.Background(), license, tt.args.channelID)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
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
						"channelSequence": 1,
						"releaseSequence": 1,
						"versionLabel": "1.0.0",
						"isRequired": true,
						"createdAt": "2023-10-01T00:00:00Z",
						"releaseNotes": "release notes"
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
						"channelSequence": 2,
						"releaseSequence": 2,
						"versionLabel": "2.0.0",
						"isRequired": true,
						"createdAt": "2023-10-01T00:00:00Z",
						"releaseNotes": "release notes"
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
						"channelSequence": 2,
						"releaseSequence": 2,
						"versionLabel": "2.0.0",
						"isRequired": true,
						"createdAt": "2023-10-01T00:00:00Z",
						"releaseNotes": "release notes"
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
				apiURL := startFakeServer(t, handler)
				license = &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						LicenseID: "license-id",
						AppSlug:   "app-slug",
						Endpoint:  apiURL.String(),
					},
				}

				embedStr := "# channel release object\nchannelID: \"%s\"\nchannelSlug: \"%s\"\nappSlug: \"%s\"\nversionLabel: \"%s\""
				releaseDataMap["release.yaml"] = []byte(fmt.Sprintf(
					embedStr,
					tt.channelRelease.ChannelID,
					tt.channelRelease.ChannelSlug,
					tt.channelRelease.AppSlug,
					tt.channelRelease.VersionLabel,
				))
				err := release.SetReleaseDataForTests(releaseDataMap)
				require.NoError(t, err)
			}
			err := release.SetReleaseDataForTests(releaseDataMap)
			require.NoError(t, err)

			var in *bytes.Buffer
			if tt.confirm {
				in = bytes.NewBuffer([]byte("y\n"))
			} else {
				in = bytes.NewBuffer([]byte("n\n"))
			}
			out := bytes.NewBuffer([]byte{})
			prompt := plain.New(plain.WithIn(in), plain.WithOut(out))

			flagset := flag.NewFlagSet("test", 0)
			err = maybePromptForAppUpdate(cli.NewContext(nil, flagset, nil), prompt, license)
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
				assert.Equal(t, ErrNothingElseToAdd, err)
			} else {
				assert.NotEqual(t, ErrNothingElseToAdd, err)
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

func startFakeServer(t *testing.T, handler http.HandlerFunc) *url.URL {
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = listener.Close()
	})

	addr := listener.Addr().(*net.TCPAddr)

	server := &http.Server{
		Addr: addr.String(),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/_ping" {
				w.WriteHeader(http.StatusOK)
				return
			}
			handler(w, r)
		}),
	}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Shutdown(context.Background())
	})

	u := &url.URL{Scheme: "http", Host: server.Addr}

	// wait for the server to start
	for i := 0; i < 10; i++ {
		if _, err := http.Get(fmt.Sprintf("http://%s/_ping", u)); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	return u
}
