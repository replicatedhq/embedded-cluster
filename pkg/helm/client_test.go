package helm

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/repo"
	k8syaml "sigs.k8s.io/yaml"
)

func TestHelmClient_PullByRef(t *testing.T) {
	tests := []struct {
		name         string
		ref          string
		version      string
		repositories []*repo.Entry
		setupMock    func(*MockBinaryExecutor)
		want         string
		wantErr      bool
	}{
		{
			name:    "successful pull with repository preparation",
			ref:     "myrepo/mychart",
			version: "1.2.3",
			repositories: []*repo.Entry{
				{
					Name: "myrepo",
					URL:  "https://charts.example.com/myrepo",
				},
			},
			setupMock: func(m *MockBinaryExecutor) {
				// Mock helm repo update command (called by prepare())
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					[]string{"repo", "update", "myrepo"},
				).Return("", "", nil)

				// Mock helm pull command
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					mock.MatchedBy(func(args []string) bool {
						return len(args) == 7 &&
							args[0] == "pull" &&
							args[1] == "myrepo/mychart" &&
							args[2] == "--version" &&
							args[3] == "1.2.3" &&
							args[4] == "--destination" &&
							// args[5] is the temp directory path, which varies
							args[6] == "--debug"
					}),
				).Return("", "", nil)

				// Mock helm show chart command for metadata
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					[]string{"show", "chart", "myrepo/mychart"},
				).Return(`apiVersion: v2
name: mychart
description: A test chart from repo
type: application
version: 1.2.3
appVersion: "1.0.0"`, "", nil)
			},
			want:    "mychart-1.2.3.tgz",
			wantErr: false,
		},
		{
			name:         "successful pull from OCI registry",
			ref:          "oci://registry.example.com/charts/nginx",
			version:      "2.1.0",
			repositories: nil, // OCI charts don't use repositories
			setupMock: func(m *MockBinaryExecutor) {
				// No helm repo update for OCI charts (prepare() is skipped)

				// Mock helm pull command for OCI
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					mock.MatchedBy(func(args []string) bool {
						return len(args) == 7 &&
							args[0] == "pull" &&
							args[1] == "oci://registry.example.com/charts/nginx" &&
							args[2] == "--version" &&
							args[3] == "2.1.0" &&
							args[4] == "--destination" &&
							// args[5] is the temp directory path, which varies
							args[6] == "--debug"
					}),
				).Return("", "", nil)

				// Mock helm show chart command for metadata
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					[]string{"show", "chart", "oci://registry.example.com/charts/nginx"},
				).Return(`apiVersion: v2
name: nginx
description: A nginx chart from OCI registry
type: application
version: 2.1.0
appVersion: "1.25.0"`, "", nil)
			},
			want:    "nginx-2.1.0.tgz",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := &MockBinaryExecutor{}
			tt.setupMock(mockExec)

			// Create temporary directory for the test
			tmpdir := t.TempDir()

			client := &HelmClient{
				helmPath:     "/usr/local/bin/helm",
				executor:     mockExec,
				tmpdir:       tmpdir,
				repositories: tt.repositories,
			}

			got, err := client.PullByRef(t.Context(), tt.ref, tt.version)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			// Check that the returned path ends with the expected filename
			assert.True(t, strings.HasSuffix(got, tt.want))
			mockExec.AssertExpectations(t)
		})
	}
}

func TestHelmClient_Install(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*MockBinaryExecutor)
		opts      InstallOptions
		wantErr   bool
	}{
		{
			name: "successful install",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					[]string{"install", "myrelease", "/path/to/chart", "--namespace", "default", "--create-namespace", "--wait", "--wait-for-jobs", "--timeout", "5m0s", "--replace", "--debug"},
				).Return(`Release "myrelease" has been upgraded.`, "", nil)
			},
			opts: InstallOptions{
				ReleaseName: "myrelease",
				ChartPath:   "/path/to/chart",
				Namespace:   "default",
				Timeout:     5 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "install with values",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					mock.MatchedBy(func(args []string) bool {
						// Check that it contains the expected arguments
						hasInstall := false
						hasValues := false
						for i, arg := range args {
							if arg == "install" && i == 0 {
								hasInstall = true
							}
							if arg == "--values" && i < len(args)-1 {
								hasValues = true
							}
						}
						return hasInstall && hasValues
					}),
				).Return(`Release "myrelease" has been installed.`, "", nil)
			},
			opts: InstallOptions{
				ReleaseName: "myrelease",
				ChartPath:   "/path/to/chart",
				Namespace:   "default",
				Timeout:     5 * time.Minute,
				Values: map[string]interface{}{
					"key": "value",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := &MockBinaryExecutor{}
			tt.setupMock(mockExec)

			// Create temporary directory for the test
			tmpdir, err := os.MkdirTemp("", "helm-test-*")
			require.NoError(t, err)
			defer os.RemoveAll(tmpdir)

			client := &HelmClient{
				helmPath: "/usr/local/bin/helm",
				executor: mockExec,
				tmpdir:   tmpdir,
			}

			stdout, err := client.Install(t.Context(), tt.opts)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, stdout)
			mockExec.AssertExpectations(t)
		})
	}
}

func TestHelmClient_ReleaseExists(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(*MockBinaryExecutor)
		namespace   string
		releaseName string
		want        bool
		wantErr     bool
	}{
		{
			name: "release exists",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					[]string{"list", "--namespace", "default", "--filter", "^myrelease$", "--output", "json"},
				).Return(`[{
					"name": "myrelease",
					"namespace": "default",
					"revision": 1,
					"updated": "2023-01-01T00:00:00Z",
					"status": "deployed",
					"chart": "test-chart-1.0.0",
					"app_version": "1.0.0"
				}]`, "", nil)
			},
			namespace:   "default",
			releaseName: "myrelease",
			want:        true,
			wantErr:     false,
		},
		{
			name: "release does not exist",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					[]string{"list", "--namespace", "default", "--filter", "^myrelease$", "--output", "json"},
				).Return(`[]`, "", nil)
			},
			namespace:   "default",
			releaseName: "myrelease",
			want:        false,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := &MockBinaryExecutor{}
			tt.setupMock(mockExec)

			client := &HelmClient{
				helmPath: "/usr/local/bin/helm",
				executor: mockExec,
			}

			exists, err := client.ReleaseExists(t.Context(), tt.namespace, tt.releaseName)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, exists)
			mockExec.AssertExpectations(t)
		})
	}
}

func TestHelmClient_GetChartMetadata(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*MockBinaryExecutor)
		chartPath string
		version   string
		wantErr   bool
	}{
		{
			name: "successful metadata retrieval",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					[]string{"show", "chart", "/path/to/chart", "--version", "1.0.0"},
				).Return(`apiVersion: v2
name: test-chart
description: A test chart
type: application
version: 1.0.0
appVersion: "1.0.0"`, "", nil)
			},
			chartPath: "/path/to/chart",
			version:   "1.0.0",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := &MockBinaryExecutor{}
			tt.setupMock(mockExec)

			client := &HelmClient{
				helmPath: "/usr/local/bin/helm",
				executor: mockExec,
			}

			metadata, err := client.GetChartMetadata(t.Context(), tt.chartPath, tt.version)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, "test-chart", metadata.Name)
			assert.Equal(t, "1.0.0", metadata.Version)
			assert.Equal(t, "1.0.0", metadata.AppVersion)
			mockExec.AssertExpectations(t)
		})
	}
}

func Test_cleanUpGenericMap(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]interface{}
		want map[string]interface{}
	}{
		{
			name: "single level map",
			in: map[string]interface{}{
				"abc":    "xyz",
				"number": 5,
				"float":  1.5,
				"bool":   true,
				"array": []interface{}{
					"element",
				},
			},
			want: map[string]interface{}{
				"abc":    "xyz",
				"number": float64(5),
				"float":  1.5,
				"bool":   true,
				"array": []interface{}{
					"element",
				},
			},
		},
		{
			name: "nested map, string keys",
			in: map[string]interface{}{
				"nest": map[string]interface{}{
					"abc":    "xyz",
					"number": 5,
					"float":  1.5,
					"bool":   true,
					"array": []interface{}{
						"element",
					},
				},
			},
			want: map[string]interface{}{
				"nest": map[string]interface{}{
					"abc":    "xyz",
					"number": float64(5),
					"float":  1.5,
					"bool":   true,
					"array": []interface{}{
						"element",
					},
				},
			},
		},
		{
			name: "nested map, interface keys", // this is what would fail previously
			in: map[string]interface{}{
				"nest": map[interface{}]interface{}{
					"abc":    "xyz",
					"number": 5,
					"float":  1.5,
					"bool":   true,
					"array": []interface{}{
						"element",
					},
				},
			},
			want: map[string]interface{}{
				"nest": map[string]interface{}{
					"abc":    "xyz",
					"number": float64(5),
					"float":  1.5,
					"bool":   true,
					"array": []interface{}{
						"element",
					},
				},
			},
		},
		{
			name: "nested map, generic map array keys",
			in: map[string]interface{}{
				"nest": map[interface{}]interface{}{
					"abc":    "xyz",
					"number": 5,
					"float":  1.5,
					"bool":   true,
					"array": []map[string]interface{}{
						{
							"name":  "example",
							"value": "true",
						},
					},
				},
			},
			want: map[string]interface{}{
				"nest": map[string]interface{}{
					"abc":    "xyz",
					"number": float64(5),
					"float":  1.5,
					"bool":   true,
					"array": []interface{}{
						map[string]interface{}{
							"name":  "example",
							"value": "true",
						},
					},
				},
			},
		},
		{
			name: "nested map, interface map array keys",
			in: map[string]interface{}{
				"nest": map[interface{}]interface{}{
					"abc":    "xyz",
					"number": 5,
					"float":  1.5,
					"bool":   true,
					"array": []map[interface{}]interface{}{
						{
							"name":  "example",
							"value": "true",
						},
					},
				},
			},
			want: map[string]interface{}{
				"nest": map[string]interface{}{
					"abc":    "xyz",
					"number": float64(5),
					"float":  1.5,
					"bool":   true,
					"array": []interface{}{
						map[string]interface{}{
							"name":  "example",
							"value": "true",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			out, err := cleanUpGenericMap(tt.in)
			req.NoError(err, "cleanUpGenericMap failed")
			req.Equal(tt.want, out)

			// ultimately helm calls k8syaml.Marshal so we must make sure that the output is compatible
			// https://github.com/helm/helm/blob/v3.17.0/pkg/chartutil/values.go#L39
			_, err = k8syaml.Marshal(out)
			req.NoError(err, "yaml marshal failed")
		})
	}
}

func TestHelmClient_Latest(t *testing.T) {
	tests := []struct {
		name      string
		reponame  string
		chart     string
		setupMock func(*MockBinaryExecutor)
		want      string
		wantErr   bool
	}{
		{
			name:     "valid JSON response",
			reponame: "myrepo",
			chart:    "mychart",
			setupMock: func(m *MockBinaryExecutor) {
				jsonOutput := `[
					{
						"name": "myrepo/mychart",
						"version": "1.2.3",
						"app_version": "1.2.3",
						"description": "A test chart"
					}
				]`
				m.On("ExecuteCommand", mock.Anything, mock.Anything,
					[]string{"search", "repo", "myrepo/mychart", "--version", ">0.0.0", "--versions", "--output", "json"}).
					Return(jsonOutput, "", nil)
			},
			want:    "1.2.3",
			wantErr: false,
		},
		{
			name:     "empty results",
			reponame: "myrepo",
			chart:    "nonexistent",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything,
					[]string{"search", "repo", "myrepo/nonexistent", "--version", ">0.0.0", "--versions", "--output", "json"}).
					Return("[]", "", nil)
			},
			want:    "",
			wantErr: true,
		},
		{
			name:     "helm command fails",
			reponame: "myrepo",
			chart:    "mychart",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything,
					[]string{"search", "repo", "myrepo/mychart", "--version", ">0.0.0", "--versions", "--output", "json"}).
					Return("", "repo not found", assert.AnError)
			},
			want:    "",
			wantErr: true,
		},
		{
			name:     "invalid JSON response",
			reponame: "myrepo",
			chart:    "mychart",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything,
					[]string{"search", "repo", "myrepo/mychart", "--version", ">0.0.0", "--versions", "--output", "json"}).
					Return("invalid json", "", nil)
			},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := &MockBinaryExecutor{}
			tt.setupMock(mockExec)

			client := &HelmClient{
				helmPath: "/usr/local/bin/helm",
				executor: mockExec,
			}

			got, err := client.Latest(t.Context(), tt.reponame, tt.chart)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			mockExec.AssertExpectations(t)
		})
	}
}
