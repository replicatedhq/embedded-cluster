package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			want: &apiChannelRelease{
				ChannelID:                "channel-id",
				ChannelSequence:          2,
				ReleaseSequence:          2,
				VersionLabel:             "2.0.0",
				IsRequired:               true,
				CreatedAt:                "2023-10-01T00:00:00Z",
				ReleaseNotes:             "release notes",
				ReplicatedRegistryDomain: "replicated.app",
				ReplicatedProxyDomain:    "replicated.app",
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
			ts := httptest.NewServer(handler)
			t.Cleanup(ts.Close)

			releaseStr := fmt.Sprintf("# channel release object\ndefaultDomains:\n  replicatedAppDomain: %s", ts.URL)
			err := release.SetReleaseDataForTests(map[string][]byte{"release.yaml": []byte(releaseStr)})
			require.NoError(t, err)

			t.Cleanup(func() {
				release.SetReleaseDataForTests(nil)
			})

			license := &kotsv1beta1.License{
				Spec: kotsv1beta1.LicenseSpec{
					LicenseID: "license-id",
					AppSlug:   "app-slug",
				},
			}

			// Wrap the license for the new API
			wrappedLicense := licensewrapper.LicenseWrapper{V1: license}

			got, err := getCurrentAppChannelRelease(context.Background(), wrappedLicense, tt.args.channelID)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
