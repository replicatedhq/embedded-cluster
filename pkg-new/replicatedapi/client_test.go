package replicatedapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kyaml "sigs.k8s.io/yaml"
)

func TestSyncLicense(t *testing.T) {
	tests := []struct {
		name                string
		license             kotsv1beta1.License
		releaseData         *release.ReleaseData
		serverHandler       func(t *testing.T) http.HandlerFunc
		wantLicenseSequence int64
		wantAppSlug         string
		wantLicenseID       string
		wantIsV1            bool
		wantIsV2            bool
		wantErr             string
	}{
		{
			name: "successful license sync",
			license: kotsv1beta1.License{
				Spec: kotsv1beta1.LicenseSpec{
					AppSlug:         "test-app",
					LicenseID:       "test-license-id",
					LicenseSequence: 5,
					ChannelID:       "test-channel-123",
					ChannelName:     "Stable",
					Channels: []kotsv1beta1.Channel{
						{
							ChannelID:   "test-channel-123",
							ChannelName: "Stable",
						},
					},
				},
			},
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					ChannelID: "test-channel-123",
				},
			},
			serverHandler: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					// Validate request
					assert.Equal(t, http.MethodGet, r.Method)
					assert.Equal(t, "/license/test-app", r.URL.Path)
					assert.Equal(t, "5", r.URL.Query().Get("licenseSequence"))
					assert.Equal(t, "test-channel-123", r.URL.Query().Get("selectedChannelId"))
					assert.Equal(t, "application/yaml", r.Header.Get("Accept"))

					// Validate auth header
					authHeader := r.Header.Get("Authorization")
					assert.NotEmpty(t, authHeader)
					assert.Contains(t, authHeader, "Basic ")

					// Return response as YAML
					resp := kotsv1beta1.License{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "kots.io/v1beta1",
							Kind:       "License",
						},
						Spec: kotsv1beta1.LicenseSpec{
							AppSlug:         "test-app",
							LicenseID:       "test-license-id",
							LicenseSequence: 6,
							CustomerName:    "Test Customer",
							ChannelID:       "test-channel-123",
							ChannelName:     "Stable",
							Channels: []kotsv1beta1.Channel{
								{
									ChannelID:   "test-channel-123",
									ChannelName: "Stable",
								},
							},
						},
					}

					w.WriteHeader(http.StatusOK)
					respBytes, err := kyaml.Marshal(resp)
					if err != nil {
						t.Fatalf("failed to marshal license: %v", err)
					}
					w.Write(respBytes)
				}
			},
			wantLicenseSequence: 6,
			wantAppSlug:         "test-app",
			wantLicenseID:       "test-license-id",
			wantIsV1:            true,
		},
		{
			name: "successful license sync with v1beta2",
			license: kotsv1beta1.License{
				Spec: kotsv1beta1.LicenseSpec{
					AppSlug:         "test-app",
					LicenseID:       "test-license-id",
					LicenseSequence: 5,
					ChannelID:       "test-channel-123",
					ChannelName:     "Stable",
					Channels: []kotsv1beta1.Channel{
						{
							ChannelID:   "test-channel-123",
							ChannelName: "Stable",
						},
					},
				},
			},
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					ChannelID: "test-channel-123",
				},
			},
			serverHandler: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					// Validate request
					assert.Equal(t, http.MethodGet, r.Method)
					assert.Equal(t, "/license/test-app", r.URL.Path)
					assert.Equal(t, "5", r.URL.Query().Get("licenseSequence"))
					assert.Equal(t, "test-channel-123", r.URL.Query().Get("selectedChannelId"))
					assert.Equal(t, "application/yaml", r.Header.Get("Accept"))

					// Validate auth header
					authHeader := r.Header.Get("Authorization")
					assert.NotEmpty(t, authHeader)
					assert.Contains(t, authHeader, "Basic ")

					// Return v1beta2 license response
					resp := `apiVersion: kots.io/v1beta2
kind: License
spec:
  licenseID: test-license-id-v2
  appSlug: test-app
  licenseSequence: 6
  customerName: Test Customer
  channelID: test-channel-123
  channelName: Stable
  channels:
    - channelID: test-channel-123
      channelName: Stable`

					w.WriteHeader(http.StatusOK)
					w.Write([]byte(resp))
				}
			},
			wantLicenseSequence: 6,
			wantAppSlug:         "test-app",
			wantLicenseID:       "test-license-id-v2",
			wantIsV2:            true,
		},
		{
			name: "returns error on 401 unauthorized",
			license: kotsv1beta1.License{
				Spec: kotsv1beta1.LicenseSpec{
					AppSlug:         "test-app",
					LicenseID:       "wrong-license-id",
					LicenseSequence: 1,
					ChannelID:       "test-channel-123",
					Channels: []kotsv1beta1.Channel{
						{
							ChannelID:   "test-channel-123",
							ChannelName: "Stable",
						},
					},
				},
			},
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					ChannelID: "test-channel-123",
				},
			},
			serverHandler: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte("unauthorized"))
				}
			},
			wantErr: "unexpected status code 401",
		},
		{
			name: "returns error on 404 not found",
			license: kotsv1beta1.License{
				Spec: kotsv1beta1.LicenseSpec{
					AppSlug:         "nonexistent-app",
					LicenseID:       "test-license-id",
					LicenseSequence: 1,
					ChannelID:       "test-channel-123",
					Channels: []kotsv1beta1.Channel{
						{
							ChannelID:   "test-channel-123",
							ChannelName: "Stable",
						},
					},
				},
			},
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					ChannelID: "test-channel-123",
				},
			},
			serverHandler: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					w.Write([]byte("license not found"))
				}
			},
			wantErr: "unexpected status code 404",
		},
		{
			name: "returns error on 500 internal server error",
			license: kotsv1beta1.License{
				Spec: kotsv1beta1.LicenseSpec{
					AppSlug:         "test-app",
					LicenseID:       "test-license-id",
					LicenseSequence: 1,
					ChannelID:       "test-channel-123",
					Channels: []kotsv1beta1.Channel{
						{
							ChannelID:   "test-channel-123",
							ChannelName: "Stable",
						},
					},
				},
			},
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					ChannelID: "test-channel-123",
				},
			},
			serverHandler: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("internal server error"))
				}
			},
			wantErr: "unexpected status code 500",
		},
		{
			name: "returns error on invalid YAML response",
			license: kotsv1beta1.License{
				Spec: kotsv1beta1.LicenseSpec{
					AppSlug:         "test-app",
					LicenseID:       "test-license-id",
					LicenseSequence: 1,
					ChannelID:       "test-channel-123",
					Channels: []kotsv1beta1.Channel{
						{
							ChannelID:   "test-channel-123",
							ChannelName: "Stable",
						},
					},
				},
			},
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					ChannelID: "test-channel-123",
				},
			},
			serverHandler: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("invalid yaml"))
				}
			},
			wantErr: "parse license response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			// Create test server
			server := httptest.NewServer(tt.serverHandler(t))
			defer server.Close()

			// Wrap the v1beta1 license first
			wrapper := &licensewrapper.LicenseWrapper{V1: &tt.license}

			// Create client with wrapper
			c, err := NewClient(server.URL, wrapper, tt.releaseData)
			req.NoError(err)

			// Execute test
			license, rawLicense, err := c.SyncLicense(context.Background())

			// Validate results
			if tt.wantErr != "" {
				req.Error(err)
				req.Contains(err.Error(), tt.wantErr)
				req.Nil(license)
				req.Nil(rawLicense)
			} else {
				req.NoError(err)
				req.NotNil(rawLicense)

				// Assert using wrapper methods (works for both v1beta1 and v1beta2)
				assert.Equal(t, tt.wantLicenseSequence, license.GetLicenseSequence())
				assert.Equal(t, tt.wantAppSlug, license.GetAppSlug())
				assert.Equal(t, tt.wantLicenseID, license.GetLicenseID())

				// Assert version
				if tt.wantIsV1 {
					assert.True(t, license.IsV1())
					assert.False(t, license.IsV2())
				}
				if tt.wantIsV2 {
					assert.False(t, license.IsV1())
					assert.True(t, license.IsV2())
				}

				// Validate raw license is valid YAML
				var parsedLicense kotsv1beta1.License
				err = kyaml.Unmarshal(rawLicense, &parsedLicense)
				req.NoError(err, "rawLicense should be valid YAML")
			}
		})
	}
}

func TestGetReportingInfoHeaders(t *testing.T) {
	tests := []struct {
		name          string
		clusterID     string
		expectedCount int
		checkHeaders  map[string]string
	}{
		{
			name:          "with cluster ID",
			clusterID:     "cluster-123",
			expectedCount: 7, // EmbeddedClusterID, ChannelID, ChannelName, K8sVersion, K8sDistribution, EmbeddedClusterVersion, IsKurl
			checkHeaders: map[string]string{
				"X-Replicated-EmbeddedClusterID":      "cluster-123",
				"X-Replicated-DownstreamChannelID":    "test-channel-123",
				"X-Replicated-DownstreamChannelName":  "Stable",
				"X-Replicated-K8sVersion":             versions.K0sVersion,
				"X-Replicated-K8sDistribution":        DistributionEmbeddedCluster,
				"X-Replicated-EmbeddedClusterVersion": versions.Version,
				"X-Replicated-IsKurl":                 "false",
			},
		},
		{
			name:          "zero values should be skipped",
			clusterID:     "",
			expectedCount: 6, // ChannelID, ChannelName, K8sVersion, K8sDistribution, EmbeddedClusterVersion, IsKurl
			checkHeaders: map[string]string{
				"X-Replicated-IsKurl": "false",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			license := kotsv1beta1.License{
				Spec: kotsv1beta1.LicenseSpec{
					AppSlug:         "test-app",
					LicenseID:       "test-license-id",
					LicenseSequence: 1,
					ChannelID:       "test-channel-123",
					ChannelName:     "Stable",
					Channels: []kotsv1beta1.Channel{
						{
							ChannelID:   "test-channel-123",
							ChannelName: "Stable",
						},
					},
				},
			}

			releaseData := &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					ChannelID: "test-channel-123",
				},
			}

			c := &client{
				license:     &licensewrapper.LicenseWrapper{V1: &license},
				releaseData: releaseData,
				clusterID:   tt.clusterID,
			}

			headers := c.getReportingInfoHeaders()

			req.Len(headers, tt.expectedCount)

			for key, expectedValue := range tt.checkHeaders {
				actualValue, exists := headers[key]
				req.True(exists, "Expected header %s to exist", key)
				req.Equal(expectedValue, actualValue, "Header %s has wrong value", key)
			}
		})
	}
}

func TestInjectHeaders(t *testing.T) {
	req := require.New(t)

	// Create client
	license := kotsv1beta1.License{
		Spec: kotsv1beta1.LicenseSpec{
			AppSlug:         "test-app",
			LicenseID:       "test-license-id",
			LicenseSequence: 1,
			ChannelID:       "test-channel-123",
			ChannelName:     "Stable",
			Channels: []kotsv1beta1.Channel{
				{
					ChannelID:   "test-channel-123",
					ChannelName: "Stable",
				},
			},
		},
	}

	releaseData := &release.ReleaseData{
		ChannelRelease: &release.ChannelRelease{
			ChannelID: "test-channel-123",
		},
	}

	c := &client{
		license:     &licensewrapper.LicenseWrapper{V1: &license},
		releaseData: releaseData,
		clusterID:   "test-cluster-id",
	}

	// Test the internal method directly
	header := http.Header{}
	c.injectHeaders(header)

	// Validate User-Agent header was injected
	userAgent := header.Get("User-Agent")
	req.NotEmpty(userAgent)
	req.Contains(userAgent, "Embedded-Cluster/")
	req.Contains(userAgent, versions.Version)

	// Validate Authorization header
	authHeader := header.Get("Authorization")
	req.NotEmpty(authHeader)
	req.Contains(authHeader, "Basic ")

	// Validate reporting info headers were injected
	req.Equal("test-cluster-id", header.Get("X-Replicated-EmbeddedClusterID"))
	req.Equal("test-channel-123", header.Get("X-Replicated-DownstreamChannelID"))
	req.Equal("Stable", header.Get("X-Replicated-DownstreamChannelName"))
	req.Equal(versions.K0sVersion, header.Get("X-Replicated-K8sVersion"))
	req.Equal(DistributionEmbeddedCluster, header.Get("X-Replicated-K8sDistribution"))
	req.Equal(versions.Version, header.Get("X-Replicated-EmbeddedClusterVersion"))
	req.Equal("false", header.Get("X-Replicated-IsKurl"))
}
