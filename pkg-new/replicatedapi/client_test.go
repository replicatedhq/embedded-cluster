package replicatedapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSyncLicense(t *testing.T) {
	tests := []struct {
		name            string
		license         kotsv1beta1.License
		releaseData     *release.ReleaseData
		serverHandler   func(t *testing.T) http.HandlerFunc
		expectedLicense *kotsv1beta1.License
		wantErr         string
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
					yaml.NewEncoder(w).Encode(resp)
				}
			},
			expectedLicense: &kotsv1beta1.License{
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
				},
			},
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
			wantErr: "unmarshal license response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			// Create test server
			server := httptest.NewServer(tt.serverHandler(t))
			defer server.Close()

			// Create client
			c, err := NewClient(server.URL, &tt.license, tt.releaseData)
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
				req.NotNil(license)
				req.NotNil(rawLicense)
				assert.Equal(t, tt.expectedLicense.Spec.AppSlug, license.Spec.AppSlug)
				assert.Equal(t, tt.expectedLicense.Spec.LicenseID, license.Spec.LicenseID)
				assert.Equal(t, tt.expectedLicense.Spec.LicenseSequence, license.Spec.LicenseSequence)

				// Validate raw license is valid YAML
				var parsedLicense kotsv1beta1.License
				err = yaml.Unmarshal(rawLicense, &parsedLicense)
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
				license:     &license,
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
		license:     &license,
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

func TestGetPendingReleases(t *testing.T) {
	tests := []struct {
		name             string
		channelID        string
		channelSequence  int64
		opts             *PendingReleasesOptions
		serverHandler    func(t *testing.T) http.HandlerFunc
		expectedResponse *PendingReleasesResponse
		wantErr          string
	}{
		{
			name:            "successful pending releases fetch with multiple releases",
			channelID:       "test-channel-123",
			channelSequence: 10,
			opts: &PendingReleasesOptions{
				IsSemverSupported: true,
				SortOrder:         SortOrderAscending,
			},
			serverHandler: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					// Validate request
					assert.Equal(t, http.MethodGet, r.Method)
					assert.Equal(t, "/release/test-app/pending", r.URL.Path)
					assert.Equal(t, "test-channel-123", r.URL.Query().Get("selectedChannelId"))
					assert.Equal(t, "10", r.URL.Query().Get("channelSequence"))
					assert.Equal(t, "true", r.URL.Query().Get("isSemverSupported"))
					assert.Equal(t, "asc", r.URL.Query().Get("sortOrder"))
					assert.Equal(t, "application/json", r.Header.Get("Accept"))

					// Validate auth header
					authHeader := r.Header.Get("Authorization")
					assert.NotEmpty(t, authHeader)
					assert.Contains(t, authHeader, "Basic ")

					// Return response as JSON
					resp := PendingReleasesResponse{
						ChannelReleases: []ChannelRelease{
							{
								ChannelID:       "test-channel-123",
								ChannelSequence: 11,
								ReleaseSequence: 101,
								VersionLabel:    "1.0.1",
								IsRequired:      false,
							},
							{
								ChannelID:       "test-channel-123",
								ChannelSequence: 12,
								ReleaseSequence: 102,
								VersionLabel:    "1.0.2",
								IsRequired:      true,
							},
							{
								ChannelID:       "test-channel-123",
								ChannelSequence: 13,
								ReleaseSequence: 103,
								VersionLabel:    "1.0.3",
								IsRequired:      false,
							},
						},
					}

					w.WriteHeader(http.StatusOK)
					yaml.NewEncoder(w).Encode(resp)
				}
			},
			expectedResponse: &PendingReleasesResponse{
				ChannelReleases: []ChannelRelease{
					{
						ChannelID:       "test-channel-123",
						ChannelSequence: 11,
						ReleaseSequence: 101,
						VersionLabel:    "1.0.1",
						IsRequired:      false,
					},
					{
						ChannelID:       "test-channel-123",
						ChannelSequence: 12,
						ReleaseSequence: 102,
						VersionLabel:    "1.0.2",
						IsRequired:      true,
					},
					{
						ChannelID:       "test-channel-123",
						ChannelSequence: 13,
						ReleaseSequence: 103,
						VersionLabel:    "1.0.3",
						IsRequired:      false,
					},
				},
			},
		},
		{
			name:            "successful pending releases fetch with empty results",
			channelID:       "test-channel-123",
			channelSequence: 10,
			opts: &PendingReleasesOptions{
				IsSemverSupported: false,
				SortOrder:         SortOrderAscending,
			},
			serverHandler: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					resp := PendingReleasesResponse{
						ChannelReleases: []ChannelRelease{},
					}
					w.WriteHeader(http.StatusOK)
					yaml.NewEncoder(w).Encode(resp)
				}
			},
			expectedResponse: &PendingReleasesResponse{
				ChannelReleases: []ChannelRelease{},
			},
		},
		{
			name:            "successful pending releases with ascending sort order",
			channelID:       "test-channel-123",
			channelSequence: 5,
			opts: &PendingReleasesOptions{
				IsSemverSupported: true,
				SortOrder:         SortOrderAscending,
			},
			serverHandler: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "asc", r.URL.Query().Get("sortOrder"))
					resp := PendingReleasesResponse{
						ChannelReleases: []ChannelRelease{
							{
								ChannelSequence: 6,
								VersionLabel:    "1.0.0",
							},
						},
					}
					w.WriteHeader(http.StatusOK)
					yaml.NewEncoder(w).Encode(resp)
				}
			},
			expectedResponse: &PendingReleasesResponse{
				ChannelReleases: []ChannelRelease{
					{
						ChannelSequence: 6,
						VersionLabel:    "1.0.0",
					},
				},
			},
		},
		{
			name:            "successful pending releases with descending sort order",
			channelID:       "test-channel-123",
			channelSequence: 5,
			opts: &PendingReleasesOptions{
				IsSemverSupported: false,
				SortOrder:         SortOrderDescending,
			},
			serverHandler: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "desc", r.URL.Query().Get("sortOrder"))
					resp := PendingReleasesResponse{
						ChannelReleases: []ChannelRelease{
							{
								ChannelSequence: 10,
								VersionLabel:    "2.0.0",
							},
						},
					}
					w.WriteHeader(http.StatusOK)
					yaml.NewEncoder(w).Encode(resp)
				}
			},
			expectedResponse: &PendingReleasesResponse{
				ChannelReleases: []ChannelRelease{
					{
						ChannelSequence: 10,
						VersionLabel:    "2.0.0",
					},
				},
			},
		},
		{
			name:            "returns error on 401 unauthorized",
			channelID:       "test-channel-123",
			channelSequence: 10,
			opts: &PendingReleasesOptions{
				IsSemverSupported: true,
				SortOrder:         SortOrderAscending,
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
			name:            "returns error on 404 not found",
			channelID:       "nonexistent-channel",
			channelSequence: 10,
			opts: &PendingReleasesOptions{
				IsSemverSupported: true,
				SortOrder:         SortOrderAscending,
			},
			serverHandler: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					w.Write([]byte("channel not found"))
				}
			},
			wantErr: "unexpected status code 404",
		},
		{
			name:            "returns error on 500 internal server error",
			channelID:       "test-channel-123",
			channelSequence: 10,
			opts: &PendingReleasesOptions{
				IsSemverSupported: true,
				SortOrder:         SortOrderAscending,
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
			name:            "returns error on invalid JSON response",
			channelID:       "test-channel-123",
			channelSequence: 10,
			opts: &PendingReleasesOptions{
				IsSemverSupported: true,
				SortOrder:         SortOrderAscending,
			},
			serverHandler: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("invalid json"))
				}
			},
			wantErr: "unmarshal pending releases response",
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

			// Create test server
			server := httptest.NewServer(tt.serverHandler(t))
			defer server.Close()

			// Create client
			c, err := NewClient(server.URL, &license, releaseData)
			req.NoError(err)

			// Execute test
			result, err := c.GetPendingReleases(context.Background(), tt.channelID, tt.channelSequence, tt.opts)

			// Validate results
			if tt.wantErr != "" {
				req.Error(err)
				req.Contains(err.Error(), tt.wantErr)
				req.Nil(result)
			} else {
				req.NoError(err)
				req.NotNil(result)
				req.Equal(len(tt.expectedResponse.ChannelReleases), len(result.ChannelReleases))

				for i, expectedRelease := range tt.expectedResponse.ChannelReleases {
					assert.Equal(t, expectedRelease.ChannelID, result.ChannelReleases[i].ChannelID)
					assert.Equal(t, expectedRelease.ChannelSequence, result.ChannelReleases[i].ChannelSequence)
					assert.Equal(t, expectedRelease.ReleaseSequence, result.ChannelReleases[i].ReleaseSequence)
					assert.Equal(t, expectedRelease.VersionLabel, result.ChannelReleases[i].VersionLabel)
					assert.Equal(t, expectedRelease.IsRequired, result.ChannelReleases[i].IsRequired)
				}
			}
		})
	}
}

func TestGetPendingReleases_ContextCancellation(t *testing.T) {
	req := require.New(t)

	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		<-r.Context().Done()
	}))
	defer server.Close()

	license := kotsv1beta1.License{
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
	}

	releaseData := &release.ReleaseData{
		ChannelRelease: &release.ChannelRelease{
			ChannelID: "test-channel-123",
		},
	}

	// Create client
	c, err := NewClient(server.URL, &license, releaseData)
	req.NoError(err)

	// Create a context that is already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	opts := &PendingReleasesOptions{
		IsSemverSupported: true,
		SortOrder:         SortOrderAscending,
	}

	// Execute test
	result, err := c.GetPendingReleases(ctx, "test-channel-123", 10, opts)

	// Should return error due to cancelled context
	req.Error(err)
	req.Nil(result)
}
