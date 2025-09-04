package helm

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	helmcli "helm.sh/helm/v3/pkg/cli"
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
					mock.Anything, // LogFn
					[]string{"repo", "update", "myrepo"},
				).Return("", "", nil)

				// Mock helm pull command
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					mock.Anything, // LogFn
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
					mock.Anything, // LogFn
					[]string{"show", "chart", "myrepo/mychart", "--version", "1.2.3"},
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
					mock.Anything, // LogFn
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
					mock.Anything, // LogFn
					[]string{"show", "chart", "oci://registry.example.com/charts/nginx", "--version", "2.1.0"},
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
		name                  string
		setupMock             func(*MockBinaryExecutor)
		kubernetesEnvSettings *helmcli.EnvSettings
		opts                  InstallOptions
		wantErr               bool
	}{
		{
			name: "successful install",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					mock.Anything, // LogFn
					[]string{"install", "myrelease", "/path/to/chart", "--namespace", "default", "--create-namespace", "--wait", "--wait-for-jobs", "--timeout", "5m0s", "--replace", "--debug"},
				).Return(`Release "myrelease" has been upgraded.`, "", nil)
			},
			kubernetesEnvSettings: nil, // No kubeconfig settings
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
					mock.Anything, // LogFn
					mock.MatchedBy(func(args []string) bool {
						argsStr := strings.Join(args, " ")
						return strings.HasPrefix(argsStr, "install") &&
							strings.Contains(argsStr, "--values")
					}),
				).Return(`Release "myrelease" has been installed.`, "", nil)
			},
			kubernetesEnvSettings: nil, // No kubeconfig settings
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

		{
			name: "install with kubernetes env settings",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					mock.Anything, // LogFn
					mock.MatchedBy(func(args []string) bool {
						argsStr := strings.Join(args, " ")
						return strings.HasPrefix(argsStr, "install") &&
							strings.Contains(argsStr, "--kubeconfig /tmp/test-kubeconfig") &&
							strings.Contains(argsStr, "--kube-context test-context") &&
							strings.Contains(argsStr, "--kube-token test-token") &&
							strings.Contains(argsStr, "--kube-as-user test-user") &&
							strings.Contains(argsStr, "--kube-as-group test-group1") &&
							strings.Contains(argsStr, "--kube-as-group test-group2") &&
							strings.Contains(argsStr, "--kube-apiserver https://test-server:6443") &&
							strings.Contains(argsStr, "--kube-ca-file /tmp/ca.crt") &&
							strings.Contains(argsStr, "--kube-tls-server-name test-server") &&
							strings.Contains(argsStr, "--kube-insecure-skip-tls-verify") &&
							strings.Contains(argsStr, "--burst-limit 100") &&
							strings.Contains(argsStr, "--qps 50.00")
					}),
				).Return(`Release "myrelease" has been installed.`, "", nil)
			},
			kubernetesEnvSettings: &helmcli.EnvSettings{
				KubeConfig:                "/tmp/test-kubeconfig",
				KubeContext:               "test-context",
				KubeToken:                 "test-token",
				KubeAsUser:                "test-user",
				KubeAsGroups:              []string{"test-group1", "test-group2"},
				KubeAPIServer:             "https://test-server:6443",
				KubeCaFile:                "/tmp/ca.crt",
				KubeTLSServerName:         "test-server",
				KubeInsecureSkipTLSVerify: true,
				BurstLimit:                100,
				QPS:                       50.0,
			},
			opts: InstallOptions{
				ReleaseName: "myrelease",
				ChartPath:   "/path/to/chart",
				Namespace:   "default",
				Timeout:     5 * time.Minute,
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
				helmPath:              "/usr/local/bin/helm",
				executor:              mockExec,
				tmpdir:                tmpdir,
				kubernetesEnvSettings: tt.kubernetesEnvSettings,
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
		name                  string
		setupMock             func(*MockBinaryExecutor)
		kubernetesEnvSettings *helmcli.EnvSettings
		namespace             string
		releaseName           string
		want                  bool
		wantErr               bool
	}{
		{
			name: "release exists",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					mock.Anything, // LogFn
					mock.MatchedBy(func(args []string) bool {
						argsStr := strings.Join(args, " ")
						return strings.HasPrefix(argsStr, "history") &&
							strings.Contains(argsStr, "myrelease") &&
							strings.Contains(argsStr, "--namespace default") &&
							strings.Contains(argsStr, "--max 1") &&
							strings.Contains(argsStr, "--output json") &&
							strings.Contains(argsStr, "--kubeconfig /tmp/test-kubeconfig")
					}),
				).Return(`[{
					"revision": 1,
					"updated": "2023-01-01T00:00:00Z",
					"status": "deployed",
					"chart": "test-chart-1.0.0",
					"app_version": "1.0.0",
					"description": "Install complete"
				}]`, "", nil)
			},
			kubernetesEnvSettings: &helmcli.EnvSettings{
				KubeConfig: "/tmp/test-kubeconfig",
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
					mock.Anything, // LogFn
					[]string{"history", "myrelease", "--namespace", "default", "--max", "1", "--output", "json"},
				).Return(`[]`, "", nil)
			},
			kubernetesEnvSettings: nil, // No kubeconfig settings
			namespace:             "default",
			releaseName:           "myrelease",
			want:                  false,
			wantErr:               false,
		},
		{
			name: "release exists but is uninstalled",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					mock.Anything, // LogFn
					mock.MatchedBy(func(args []string) bool {
						argsStr := strings.Join(args, " ")
						return strings.HasPrefix(argsStr, "history") &&
							strings.Contains(argsStr, "myrelease") &&
							strings.Contains(argsStr, "--namespace default") &&
							strings.Contains(argsStr, "--max 1") &&
							strings.Contains(argsStr, "--output json")
					}),
				).Return(`[{
					"revision": 2,
					"updated": "2023-01-01T01:00:00Z",
					"status": "uninstalled",
					"chart": "test-chart-1.0.0",
					"app_version": "1.0.0",
					"description": "Uninstallation complete"
				}]`, "", nil)
			},
			kubernetesEnvSettings: nil,
			namespace:             "default",
			releaseName:           "myrelease",
			want:                  false,
			wantErr:               false,
		},
		{
			name: "release exists in pending-install state",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					mock.Anything, // LogFn
					mock.MatchedBy(func(args []string) bool {
						argsStr := strings.Join(args, " ")
						return strings.HasPrefix(argsStr, "history") &&
							strings.Contains(argsStr, "myrelease") &&
							strings.Contains(argsStr, "--namespace default") &&
							strings.Contains(argsStr, "--max 1") &&
							strings.Contains(argsStr, "--output json")
					}),
				).Return(`[{
					"revision": 1,
					"updated": "2023-01-01T00:00:00Z",
					"status": "pending-install",
					"chart": "test-chart-1.0.0",
					"app_version": "1.0.0",
					"description": "Install in progress"
				}]`, "", nil)
			},
			kubernetesEnvSettings: nil,
			namespace:             "default",
			releaseName:           "myrelease",
			want:                  true,
			wantErr:               false,
		},
		{
			name: "release not found error in stderr",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					mock.Anything, // LogFn
					mock.MatchedBy(func(args []string) bool {
						argsStr := strings.Join(args, " ")
						return strings.HasPrefix(argsStr, "history") &&
							strings.Contains(argsStr, "myrelease")
					}),
				).Return("", "release: not found", fmt.Errorf("exit status 1"))
			},
			kubernetesEnvSettings: nil,
			namespace:             "default",
			releaseName:           "myrelease",
			want:                  false,
			wantErr:               false,
		},
		{
			name: "release not found error in err message",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					mock.Anything, // LogFn
					mock.MatchedBy(func(args []string) bool {
						argsStr := strings.Join(args, " ")
						return strings.HasPrefix(argsStr, "history") &&
							strings.Contains(argsStr, "myrelease")
					}),
				).Return("", "", fmt.Errorf("release: not found"))
			},
			kubernetesEnvSettings: nil,
			namespace:             "default",
			releaseName:           "myrelease",
			want:                  false,
			wantErr:               false,
		},
		{
			name: "other command execution error",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					mock.Anything, // LogFn
					mock.MatchedBy(func(args []string) bool {
						argsStr := strings.Join(args, " ")
						return strings.HasPrefix(argsStr, "history") &&
							strings.Contains(argsStr, "myrelease")
					}),
				).Return("", "connection refused", fmt.Errorf("exit status 1"))
			},
			kubernetesEnvSettings: nil,
			namespace:             "default",
			releaseName:           "myrelease",
			want:                  false,
			wantErr:               true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := &MockBinaryExecutor{}
			tt.setupMock(mockExec)

			client := &HelmClient{
				helmPath:              "/usr/local/bin/helm",
				executor:              mockExec,
				kubernetesEnvSettings: tt.kubernetesEnvSettings,
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
					mock.Anything, // LogFn
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
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
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
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
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
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
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
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
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

func TestHelmClient_Upgrade(t *testing.T) {
	tests := []struct {
		name                  string
		setupMock             func(*MockBinaryExecutor)
		kubernetesEnvSettings *helmcli.EnvSettings
		opts                  UpgradeOptions
		wantErr               bool
	}{
		{
			name: "successful upgrade",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					mock.Anything, // LogFn
					[]string{"upgrade", "myrelease", "/path/to/chart", "--namespace", "default", "--wait", "--wait-for-jobs", "--timeout", "5m0s", "--atomic", "--debug"},
				).Return(`Release "myrelease" has been upgraded.`, "", nil)
			},
			kubernetesEnvSettings: nil, // No kubeconfig settings
			opts: UpgradeOptions{
				ReleaseName: "myrelease",
				ChartPath:   "/path/to/chart",
				Namespace:   "default",
				Timeout:     5 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "upgrade with values",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					mock.Anything, // LogFn
					mock.MatchedBy(func(args []string) bool {
						argsStr := strings.Join(args, " ")
						return strings.HasPrefix(argsStr, "upgrade") &&
							strings.Contains(argsStr, "--values")
					}),
				).Return(`Release "myrelease" has been upgraded.`, "", nil)
			},
			kubernetesEnvSettings: nil, // No kubeconfig settings
			opts: UpgradeOptions{
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
		{
			name: "upgrade with kubernetes env settings",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					mock.Anything, // LogFn
					mock.MatchedBy(func(args []string) bool {
						argsStr := strings.Join(args, " ")
						return strings.HasPrefix(argsStr, "upgrade") &&
							strings.Contains(argsStr, "--kubeconfig /tmp/test-kubeconfig") &&
							strings.Contains(argsStr, "--kube-context test-context") &&
							strings.Contains(argsStr, "--kube-token test-token") &&
							strings.Contains(argsStr, "--kube-as-user test-user") &&
							strings.Contains(argsStr, "--kube-as-group test-group1") &&
							strings.Contains(argsStr, "--kube-as-group test-group2") &&
							strings.Contains(argsStr, "--kube-apiserver https://test-server:6443") &&
							strings.Contains(argsStr, "--kube-ca-file /tmp/ca.crt") &&
							strings.Contains(argsStr, "--kube-tls-server-name test-server") &&
							strings.Contains(argsStr, "--kube-insecure-skip-tls-verify") &&
							strings.Contains(argsStr, "--burst-limit 100") &&
							strings.Contains(argsStr, "--qps 50.00")
					}),
				).Return(`Release "myrelease" has been upgraded.`, "", nil)
			},
			kubernetesEnvSettings: &helmcli.EnvSettings{
				KubeConfig:                "/tmp/test-kubeconfig",
				KubeContext:               "test-context",
				KubeToken:                 "test-token",
				KubeAsUser:                "test-user",
				KubeAsGroups:              []string{"test-group1", "test-group2"},
				KubeAPIServer:             "https://test-server:6443",
				KubeCaFile:                "/tmp/ca.crt",
				KubeTLSServerName:         "test-server",
				KubeInsecureSkipTLSVerify: true,
				BurstLimit:                100,
				QPS:                       50.0,
			},
			opts: UpgradeOptions{
				ReleaseName: "myrelease",
				ChartPath:   "/path/to/chart",
				Namespace:   "default",
				Timeout:     5 * time.Minute,
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
				helmPath:              "/usr/local/bin/helm",
				executor:              mockExec,
				tmpdir:                tmpdir,
				kubernetesEnvSettings: tt.kubernetesEnvSettings,
			}

			_, err = client.Upgrade(t.Context(), tt.opts)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			mockExec.AssertExpectations(t)
		})
	}
}

func TestHelmClient_Uninstall(t *testing.T) {
	tests := []struct {
		name                  string
		setupMock             func(*MockBinaryExecutor)
		kubernetesEnvSettings *helmcli.EnvSettings
		opts                  UninstallOptions
		wantErr               bool
	}{
		{
			name: "successful uninstall",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					mock.Anything, // LogFn
					[]string{"uninstall", "myrelease", "--namespace", "default", "--debug"},
				).Return(`release "myrelease" uninstalled`, "", nil)
			},
			kubernetesEnvSettings: nil, // No kubeconfig settings
			opts: UninstallOptions{
				ReleaseName: "myrelease",
				Namespace:   "default",
			},
			wantErr: false,
		},
		{
			name: "uninstall with kubernetes env settings",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					mock.Anything, // LogFn
					mock.MatchedBy(func(args []string) bool {
						argsStr := strings.Join(args, " ")
						return strings.HasPrefix(argsStr, "uninstall") &&
							strings.Contains(argsStr, "--kubeconfig /tmp/test-kubeconfig") &&
							strings.Contains(argsStr, "--kube-context test-context")
					}),
				).Return(`release "myrelease" uninstalled`, "", nil)
			},
			kubernetesEnvSettings: &helmcli.EnvSettings{
				KubeConfig:  "/tmp/test-kubeconfig",
				KubeContext: "test-context",
			},
			opts: UninstallOptions{
				ReleaseName: "myrelease",
				Namespace:   "default",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := &MockBinaryExecutor{}
			tt.setupMock(mockExec)

			client := &HelmClient{
				helmPath:              "/usr/local/bin/helm",
				executor:              mockExec,
				kubernetesEnvSettings: tt.kubernetesEnvSettings,
			}

			err := client.Uninstall(t.Context(), tt.opts)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			mockExec.AssertExpectations(t)
		})
	}
}

func TestHelmClient_Render(t *testing.T) {
	tests := []struct {
		name                  string
		setupMock             func(*MockBinaryExecutor)
		kubernetesEnvSettings *helmcli.EnvSettings
		opts                  InstallOptions
		wantErr               bool
	}{
		{
			name: "successful render",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					mock.Anything, // LogFn
					[]string{"template", "myrelease", "/path/to/chart", "--namespace", "default", "--create-namespace", "--include-crds", "--debug"},
				).Return(`---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config`, "", nil)
			},
			kubernetesEnvSettings: nil, // No kubeconfig settings
			opts: InstallOptions{
				ReleaseName: "myrelease",
				ChartPath:   "/path/to/chart",
				Namespace:   "default",
			},
			wantErr: false,
		},
		{
			name: "render with values",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					mock.Anything, // LogFn
					mock.MatchedBy(func(args []string) bool {
						argsStr := strings.Join(args, " ")
						return strings.HasPrefix(argsStr, "template") &&
							strings.Contains(argsStr, "--values")
					}),
				).Return(`---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config`, "", nil)
			},
			kubernetesEnvSettings: nil, // No kubeconfig settings
			opts: InstallOptions{
				ReleaseName: "myrelease",
				ChartPath:   "/path/to/chart",
				Namespace:   "default",
				Values: map[string]interface{}{
					"key": "value",
				},
			},
			wantErr: false,
		},
		{
			name: "render with kubernetes env settings",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything, // context
					mock.Anything, // env
					mock.Anything, // LogFn
					mock.MatchedBy(func(args []string) bool {
						argsStr := strings.Join(args, " ")
						return strings.HasPrefix(argsStr, "template") &&
							strings.Contains(argsStr, "--kubeconfig /tmp/test-kubeconfig") &&
							strings.Contains(argsStr, "--kube-context test-context") &&
							strings.Contains(argsStr, "--kube-token test-token") &&
							strings.Contains(argsStr, "--kube-as-user test-user") &&
							strings.Contains(argsStr, "--kube-as-group test-group1") &&
							strings.Contains(argsStr, "--kube-as-group test-group2") &&
							strings.Contains(argsStr, "--kube-apiserver https://test-server:6443") &&
							strings.Contains(argsStr, "--kube-ca-file /tmp/ca.crt") &&
							strings.Contains(argsStr, "--kube-tls-server-name test-server") &&
							strings.Contains(argsStr, "--kube-insecure-skip-tls-verify") &&
							strings.Contains(argsStr, "--burst-limit 100") &&
							strings.Contains(argsStr, "--qps 50.00")
					}),
				).Return(`---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config`, "", nil)
			},
			kubernetesEnvSettings: &helmcli.EnvSettings{
				KubeConfig:                "/tmp/test-kubeconfig",
				KubeContext:               "test-context",
				KubeToken:                 "test-token",
				KubeAsUser:                "test-user",
				KubeAsGroups:              []string{"test-group1", "test-group2"},
				KubeAPIServer:             "https://test-server:6443",
				KubeCaFile:                "/tmp/ca.crt",
				KubeTLSServerName:         "test-server",
				KubeInsecureSkipTLSVerify: true,
				BurstLimit:                100,
				QPS:                       50.0,
			},
			opts: InstallOptions{
				ReleaseName: "myrelease",
				ChartPath:   "/path/to/chart",
				Namespace:   "default",
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
				helmPath:              "/usr/local/bin/helm",
				executor:              mockExec,
				tmpdir:                tmpdir,
				kubernetesEnvSettings: tt.kubernetesEnvSettings,
			}

			_, err = client.Render(t.Context(), tt.opts)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			mockExec.AssertExpectations(t)
		})
	}
}
