package lint

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIClient_GetCustomDomains(t *testing.T) {
	tests := []struct {
		name            string
		apiToken        string
		apiOrigin       string
		appID           string
		setupServer     func(*testing.T) *httptest.Server
		expectedDomains []string
		expectError     bool
		errorMsg        string
	}{
		{
			name:     "successful fetch from channel releases",
			apiToken: "test-token",
			appID:    "test-app",
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Check authorization header
					assert.Equal(t, "test-token", r.Header.Get("Authorization"))

					switch r.URL.Path {
					case "/v3/app/test-app/channels":
						response := map[string]interface{}{
							"channels": []map[string]string{
								{"id": "channel-1", "name": "Stable"},
								{"id": "channel-2", "name": "Beta"},
							},
						}
						json.NewEncoder(w).Encode(response)
					case "/v3/app/test-app/channel/channel-1/releases", "/v3/app/test-app/channel/channel-2/releases":
						response := ChannelReleasesResponse{
							ChannelReleases: []ChannelRelease{
								{
									ChannelID:       "channel-1",
									ReleaseSequence: 1,
									DefaultDomains: &Domains{
										ReplicatedApp:      "custom.example.com",
										ProxyRegistry:      "proxy.example.com",
										ReplicatedRegistry: "registry.example.com",
									},
								},
							},
						}
						json.NewEncoder(w).Encode(response)
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
			},
			expectedDomains: []string{"custom.example.com", "proxy.example.com", "registry.example.com"},
			expectError:     false,
		},
		{
			name:     "fallback to custom-hostnames endpoint",
			apiToken: "test-token",
			appID:    "test-app",
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v3/app/test-app/channels":
						// Return empty channels
						response := map[string]interface{}{
							"channels": []interface{}{},
						}
						json.NewEncoder(w).Encode(response)
					case "/v3/app/test-app/custom-hostnames":
						response := CustomDomainsResponse{
							Domains: []DomainInfo{
								{Domain: "app.custom.io", Type: "replicated_app"},
								{Domain: "proxy.custom.io", Type: "proxy_registry"},
							},
						}
						json.NewEncoder(w).Encode(response)
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
			},
			expectedDomains: []string{"app.custom.io", "proxy.custom.io"},
			expectError:     false,
		},
		{
			name:     "fallback to app endpoint",
			apiToken: "test-token",
			appID:    "test-app",
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v3/app/test-app/channels":
						response := map[string]interface{}{
							"channels": []interface{}{},
						}
						json.NewEncoder(w).Encode(response)
					case "/v3/app/test-app/custom-hostnames":
						w.WriteHeader(http.StatusNotFound)
					case "/v3/app/test-app":
						response := map[string]interface{}{
							"app": map[string]interface{}{
								"custom_domains": map[string]string{
									"replicated_app":      "app.domain.com",
									"proxy_registry":      "proxy.domain.com",
									"replicated_registry": "registry.domain.com",
								},
							},
						}
						json.NewEncoder(w).Encode(response)
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
			},
			expectedDomains: []string{"app.domain.com", "proxy.domain.com", "registry.domain.com"},
			expectError:     false,
		},
		{
			name:     "API returns array of strings directly",
			apiToken: "test-token",
			appID:    "test-app",
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v3/app/test-app/channels":
						response := map[string]interface{}{
							"channels": []interface{}{},
						}
						json.NewEncoder(w).Encode(response)
					case "/v3/app/test-app/custom-hostnames":
						domains := []string{"domain1.com", "domain2.com", "domain3.com"}
						json.NewEncoder(w).Encode(domains)
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
			},
			expectedDomains: []string{"domain1.com", "domain2.com", "domain3.com"},
			expectError:     false,
		},
		{
			name:     "API error response",
			apiToken: "test-token",
			appID:    "test-app",
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v3/app/test-app/channels":
						response := map[string]interface{}{
							"channels": []interface{}{},
						}
						json.NewEncoder(w).Encode(response)
					case "/v3/app/test-app/custom-hostnames":
						w.WriteHeader(http.StatusNotFound)
					case "/v3/app/test-app":
						w.WriteHeader(http.StatusInternalServerError)
						w.Write([]byte("Internal Server Error"))
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
			},
			expectedDomains: nil,
			expectError:     true,
			errorMsg:        "API request failed with status 500",
		},
		{
			name:            "missing configuration",
			apiToken:        "",
			apiOrigin:       "",
			appID:           "",
			setupServer:     nil,
			expectedDomains: nil,
			expectError:     true,
			errorMsg:        "API client not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			apiOrigin := tt.apiOrigin

			if tt.setupServer != nil {
				server = tt.setupServer(t)
				defer server.Close()
				apiOrigin = server.URL
			}

			client := NewAPIClient(tt.apiToken, apiOrigin, tt.appID)

			domains, err := client.GetCustomDomains()

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.ElementsMatch(t, tt.expectedDomains, domains)
			}
		})
	}
}

func TestAPIClient_isConfigured(t *testing.T) {
	tests := []struct {
		name      string
		apiToken  string
		apiOrigin string
		appID     string
		expected  bool
	}{
		{
			name:      "fully configured",
			apiToken:  "token",
			apiOrigin: "https://api.replicated.com/vendor",
			appID:     "app-id",
			expected:  true,
		},
		{
			name:      "missing token",
			apiToken:  "",
			apiOrigin: "https://api.replicated.com/vendor",
			appID:     "app-id",
			expected:  false,
		},
		{
			name:      "missing origin",
			apiToken:  "token",
			apiOrigin: "",
			appID:     "app-id",
			expected:  false,
		},
		{
			name:      "missing app ID",
			apiToken:  "token",
			apiOrigin: "https://api.replicated.com/vendor",
			appID:     "",
			expected:  false,
		},
		{
			name:      "all missing",
			apiToken:  "",
			apiOrigin: "",
			appID:     "",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewAPIClient(tt.apiToken, tt.apiOrigin, tt.appID)
			assert.Equal(t, tt.expected, client.isConfigured())
		})
	}
}

func TestNewAPIClient(t *testing.T) {
	tests := []struct {
		name           string
		apiToken       string
		apiOrigin      string
		appID          string
		expectedOrigin string
	}{
		{
			name:           "origin without trailing slash",
			apiToken:       "token",
			apiOrigin:      "https://api.replicated.com/vendor",
			appID:          "app-id",
			expectedOrigin: "https://api.replicated.com/vendor",
		},
		{
			name:           "origin with trailing slash",
			apiToken:       "token",
			apiOrigin:      "https://api.replicated.com/vendor/",
			appID:          "app-id",
			expectedOrigin: "https://api.replicated.com/vendor",
		},
		{
			name:           "origin with multiple trailing slashes",
			apiToken:       "token",
			apiOrigin:      "https://api.replicated.com/vendor///",
			appID:          "app-id",
			expectedOrigin: "https://api.replicated.com/vendor//",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewAPIClient(tt.apiToken, tt.apiOrigin, tt.appID)
			assert.Equal(t, tt.apiToken, client.apiToken)
			assert.Equal(t, tt.expectedOrigin, client.apiOrigin)
			assert.Equal(t, tt.appID, client.appID)
			assert.NotNil(t, client.client)
		})
	}
}
