package helm

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/repo"
	k8syaml "sigs.k8s.io/yaml"
)

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

func TestHelmClient_AddRepoBin(t *testing.T) {
	tests := []struct {
		name      string
		entry     *repo.Entry
		setupMock func(*MockBinaryExecutor)
		wantErr   bool
	}{
		{
			name:  "basic repo add",
			entry: &repo.Entry{Name: "myrepo", URL: "https://charts.example.com"},
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					[]string{"repo", "add", "myrepo", "https://charts.example.com"}).
					Return("", "", nil)
			},
		},
		{
			name:  "repo add with credentials",
			entry: &repo.Entry{Name: "privaterepo", URL: "https://charts.example.com", Username: "user", Password: "pass"},
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					[]string{"repo", "add", "privaterepo", "https://charts.example.com", "--username", "user", "--password", "pass"}).
					Return("", "", nil)
			},
		},
		{
			name:  "repo add with insecure and pass-credentials",
			entry: &repo.Entry{Name: "insecurerepo", URL: "https://charts.example.com", InsecureSkipTLSverify: true, PassCredentialsAll: true},
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					[]string{"repo", "add", "insecurerepo", "https://charts.example.com", "--insecure-skip-tls-verify", "--pass-credentials"}).
					Return("", "", nil)
			},
		},
		{
			name:  "helm command fails",
			entry: &repo.Entry{Name: "myrepo", URL: "https://charts.example.com"},
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					[]string{"repo", "add", "myrepo", "https://charts.example.com"}).
					Return("", "error", assert.AnError)
			},
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

			err := client.AddRepoBin(t.Context(), tt.entry)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			mockExec.AssertExpectations(t)
		})
	}
}

func TestHelmClient_ReleaseExists(t *testing.T) {
	tests := []struct {
		name        string
		namespace   string
		releaseName string
		setupMock   func(*MockBinaryExecutor)
		want        bool
		wantErr     bool
	}{
		{
			name:        "release exists and is deployed",
			namespace:   "default",
			releaseName: "myrelease",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					[]string{"history", "myrelease", "--namespace", "default", "--max", "1", "--output", "json"}).
					Return(`[{"revision":1,"updated":"2023-01-01 00:00:00 +0000 UTC","status":"deployed","chart":"mychart-1.0.0","app_version":"1.0.0","description":"Install complete"}]`, "", nil)
			},
			want: true,
		},
		{
			name:        "release is uninstalled",
			namespace:   "default",
			releaseName: "myrelease",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					[]string{"history", "myrelease", "--namespace", "default", "--max", "1", "--output", "json"}).
					Return(`[{"revision":1,"updated":"2023-01-01 00:00:00 +0000 UTC","status":"uninstalled","chart":"mychart-1.0.0","app_version":"1.0.0","description":"Uninstallation complete"}]`, "", nil)
			},
			want: false,
		},
		{
			name:        "release not found via stderr",
			namespace:   "default",
			releaseName: "myrelease",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					[]string{"history", "myrelease", "--namespace", "default", "--max", "1", "--output", "json"}).
					Return("", "Error: release: not found", errors.New("exit status 1"))
			},
			want: false,
		},
		{
			name:        "release not found via error message",
			namespace:   "default",
			releaseName: "myrelease",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					[]string{"history", "myrelease", "--namespace", "default", "--max", "1", "--output", "json"}).
					Return("", "", errors.New("release: not found"))
			},
			want: false,
		},
		{
			name:        "helm command fails with other error",
			namespace:   "default",
			releaseName: "myrelease",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					[]string{"history", "myrelease", "--namespace", "default", "--max", "1", "--output", "json"}).
					Return("", "connection refused", errors.New("exit status 1"))
			},
			wantErr: true,
		},
		{
			name:        "empty history result",
			namespace:   "default",
			releaseName: "myrelease",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					[]string{"history", "myrelease", "--namespace", "default", "--max", "1", "--output", "json"}).
					Return("[]", "", nil)
			},
			want: false,
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

			got, err := client.ReleaseExists(t.Context(), tt.namespace, tt.releaseName)
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

func TestHelmClient_Uninstall(t *testing.T) {
	tests := []struct {
		name      string
		opts      UninstallOptions
		setupMock func(*MockBinaryExecutor)
		wantErr   bool
	}{
		{
			name: "basic uninstall",
			opts: UninstallOptions{
				ReleaseName: "myrelease",
				Namespace:   "default",
			},
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					[]string{"uninstall", "myrelease", "--namespace", "default"}).
					Return("", "", nil)
			},
		},
		{
			name: "uninstall with wait",
			opts: UninstallOptions{
				ReleaseName: "myrelease",
				Namespace:   "default",
				Wait:        true,
			},
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					[]string{"uninstall", "myrelease", "--namespace", "default", "--wait"}).
					Return("", "", nil)
			},
		},
		{
			name: "uninstall not found ignored",
			opts: UninstallOptions{
				ReleaseName:    "myrelease",
				Namespace:      "default",
				IgnoreNotFound: true,
			},
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					[]string{"uninstall", "myrelease", "--namespace", "default"}).
					Return("", "Error: release: not found", errors.New("exit status 1"))
			},
		},
		{
			name: "uninstall not found not ignored returns error",
			opts: UninstallOptions{
				ReleaseName:    "myrelease",
				Namespace:      "default",
				IgnoreNotFound: false,
			},
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					[]string{"uninstall", "myrelease", "--namespace", "default"}).
					Return("", "Error: release: not found", errors.New("exit status 1"))
			},
			wantErr: true,
		},
		{
			name: "uninstall helm command fails with other error",
			opts: UninstallOptions{
				ReleaseName: "myrelease",
				Namespace:   "default",
			},
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					[]string{"uninstall", "myrelease", "--namespace", "default"}).
					Return("", "connection refused", errors.New("exit status 1"))
			},
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

func TestHelmClient_Install(t *testing.T) {
	tmpChart, err := os.MkdirTemp("", "helm-test-chart-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpChart)

	tests := []struct {
		name      string
		opts      InstallOptions
		setupMock func(*MockBinaryExecutor)
		wantErr   bool
	}{
		{
			name: "install with local chart path",
			opts: InstallOptions{
				ReleaseName: "myrelease",
				ChartPath:   tmpChart,
				Namespace:   "default",
			},
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					mock.MatchedBy(func(args []string) bool {
						return args[0] == "install" &&
							args[1] == "myrelease" &&
							args[2] == tmpChart &&
							containsArg(args, "--namespace") &&
							containsArg(args, "--create-namespace") &&
							containsArg(args, "--wait") &&
							containsArg(args, "--wait-for-jobs") &&
							containsArg(args, "--replace")
					})).
					Return("", "", nil)
			},
		},
		{
			name: "install with remote chart includes version flag",
			opts: InstallOptions{
				ReleaseName:  "myrelease",
				ChartPath:    "myrepo/mychart",
				ChartVersion: "1.0.0",
				Namespace:    "default",
			},
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					mock.MatchedBy(func(args []string) bool {
						return args[0] == "install" &&
							args[2] == "myrepo/mychart" &&
							containsArgPair(args, "--version", "1.0.0")
					})).
					Return("", "", nil)
			},
		},
		{
			name: "install with labels",
			opts: InstallOptions{
				ReleaseName: "myrelease",
				ChartPath:   tmpChart,
				Namespace:   "default",
				Labels:      map[string]string{"env": "prod"},
			},
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					mock.MatchedBy(func(args []string) bool {
						return containsArgPair(args, "--labels", "env=prod")
					})).
					Return("", "", nil)
			},
		},
		{
			name: "install returns nil release on success",
			opts: InstallOptions{
				ReleaseName: "myrelease",
				ChartPath:   tmpChart,
				Namespace:   "default",
			},
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return("", "", nil)
			},
		},
		{
			name: "install helm command fails",
			opts: InstallOptions{
				ReleaseName: "myrelease",
				ChartPath:   tmpChart,
				Namespace:   "default",
			},
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return("", "failed", assert.AnError)
			},
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

			rel, err := client.Install(t.Context(), tt.opts)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Nil(t, rel)
			mockExec.AssertExpectations(t)
		})
	}
}

func TestHelmClient_Install_WithValues(t *testing.T) {
	tmpChart, err := os.MkdirTemp("", "helm-test-chart-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpChart)

	mockExec := &MockBinaryExecutor{}
	mockExec.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
		mock.MatchedBy(func(args []string) bool {
			for i, arg := range args {
				if arg == "--values" && i+1 < len(args) {
					content, err := os.ReadFile(args[i+1])
					if err != nil {
						return false
					}
					return strings.Contains(string(content), "key: value")
				}
			}
			return false
		})).
		Return("", "", nil)

	client := &HelmClient{
		helmPath: "/usr/local/bin/helm",
		executor: mockExec,
	}

	_, err = client.Install(context.Background(), InstallOptions{
		ReleaseName: "myrelease",
		ChartPath:   tmpChart,
		Namespace:   "default",
		Values:      map[string]any{"key": "value"},
	})
	require.NoError(t, err)
	mockExec.AssertExpectations(t)
}

func TestHelmClient_Install_AirgapPath(t *testing.T) {
	tmpAirgap, err := os.MkdirTemp("", "helm-airgap-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpAirgap)

	expectedChart := tmpAirgap + "/myrelease-1.0.0.tgz"

	mockExec := &MockBinaryExecutor{}
	mockExec.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
		mock.MatchedBy(func(args []string) bool {
			return args[0] == "install" && args[2] == expectedChart
		})).
		Return("", "", nil)

	client := &HelmClient{
		helmPath:   "/usr/local/bin/helm",
		executor:   mockExec,
		airgapPath: tmpAirgap,
	}

	_, err = client.Install(context.Background(), InstallOptions{
		ReleaseName:  "myrelease",
		ChartPath:    "myrepo/mychart",
		ChartVersion: "1.0.0",
		Namespace:    "default",
	})
	require.NoError(t, err)
	mockExec.AssertExpectations(t)
}

func TestHelmClient_Upgrade(t *testing.T) {
	tmpChart, err := os.MkdirTemp("", "helm-test-chart-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpChart)

	tests := []struct {
		name      string
		opts      UpgradeOptions
		setupMock func(*MockBinaryExecutor)
		wantErr   bool
	}{
		{
			name: "upgrade with local chart includes atomic and install flags",
			opts: UpgradeOptions{
				ReleaseName: "myrelease",
				ChartPath:   tmpChart,
				Namespace:   "default",
			},
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					mock.MatchedBy(func(args []string) bool {
						return args[0] == "upgrade" &&
							args[1] == "myrelease" &&
							args[2] == tmpChart &&
							containsArg(args, "--atomic") &&
							containsArg(args, "--install")
					})).
					Return("", "", nil)
			},
		},
		{
			name: "upgrade with force flag",
			opts: UpgradeOptions{
				ReleaseName: "myrelease",
				ChartPath:   tmpChart,
				Namespace:   "default",
				Force:       true,
			},
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					mock.MatchedBy(func(args []string) bool {
						return containsArg(args, "--force")
					})).
					Return("", "", nil)
			},
		},
		{
			name: "upgrade returns nil release on success",
			opts: UpgradeOptions{
				ReleaseName: "myrelease",
				ChartPath:   tmpChart,
				Namespace:   "default",
			},
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return("", "", nil)
			},
		},
		{
			name: "upgrade helm command fails",
			opts: UpgradeOptions{
				ReleaseName: "myrelease",
				ChartPath:   tmpChart,
				Namespace:   "default",
			},
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return("", "failed", assert.AnError)
			},
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

			rel, err := client.Upgrade(t.Context(), tt.opts)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Nil(t, rel)
			mockExec.AssertExpectations(t)
		})
	}
}

func TestHelmClient_Render(t *testing.T) {
	tmpChart, err := os.MkdirTemp("", "helm-test-chart-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpChart)

	manifestOutput := strings.Join([]string{
		"---",
		"apiVersion: v1",
		"kind: ConfigMap",
		"metadata:",
		"  name: test",
		"---",
		"apiVersion: v1",
		"kind: Service",
		"metadata:",
		"  name: test-svc",
	}, "\n")

	tests := []struct {
		name          string
		opts          InstallOptions
		setupMock     func(*MockBinaryExecutor)
		wantManifests int
		wantErr       bool
	}{
		{
			name: "render with local chart returns manifests",
			opts: InstallOptions{
				ReleaseName: "myrelease",
				ChartPath:   tmpChart,
				Namespace:   "default",
			},
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					mock.MatchedBy(func(args []string) bool {
						return args[0] == "template" &&
							args[1] == "myrelease" &&
							args[2] == tmpChart &&
							containsArg(args, "--include-crds")
					})).
					Return(manifestOutput, "", nil)
			},
			wantManifests: 2,
		},
		{
			name: "render does not include kube env args",
			opts: InstallOptions{
				ReleaseName: "myrelease",
				ChartPath:   tmpChart,
				Namespace:   "default",
			},
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					mock.MatchedBy(func(args []string) bool {
						// --kubeconfig should NOT be present for Render
						return !containsArg(args, "--kubeconfig")
					})).
					Return(manifestOutput, "", nil)
			},
			wantManifests: 2,
		},
		{
			name: "render helm command fails",
			opts: InstallOptions{
				ReleaseName: "myrelease",
				ChartPath:   tmpChart,
				Namespace:   "default",
			},
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return("", "failed", assert.AnError)
			},
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

			manifests, err := client.Render(t.Context(), tt.opts)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, manifests, tt.wantManifests)
			mockExec.AssertExpectations(t)
		})
	}
}

func TestHelmClient_Render_KubeVersion(t *testing.T) {
	tmpChart, err := os.MkdirTemp("", "helm-test-chart-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpChart)

	mockExec := &MockBinaryExecutor{}
	mockExec.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
		mock.MatchedBy(func(args []string) bool {
			return containsArgPair(args, "--kube-version", "1.28")
		})).
		Return("", "", nil)

	sv, err := semver.NewVersion("1.28.0")
	require.NoError(t, err)

	client := &HelmClient{
		helmPath: "/usr/local/bin/helm",
		executor: mockExec,
		kversion: sv,
	}

	_, err = client.Render(t.Context(), InstallOptions{
		ReleaseName: "myrelease",
		ChartPath:   tmpChart,
		Namespace:   "default",
	})
	require.NoError(t, err)
	mockExec.AssertExpectations(t)
}

func TestHelmClient_PullByRef(t *testing.T) {
	tests := []struct {
		name      string
		ref       string
		version   string
		setupMock func(*MockBinaryExecutor)
		wantErr   bool
	}{
		{
			name:    "pull includes version and destination flags",
			ref:     "myrepo/mychart",
			version: "1.0.0",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					mock.MatchedBy(func(args []string) bool {
						return args[0] == "pull" &&
							args[1] == "myrepo/mychart" &&
							containsArgPair(args, "--version", "1.0.0") &&
							containsArg(args, "--destination")
					})).
					Return("", "", nil)
			},
			// No .tgz in temp dir, so expect an error finding the file
			wantErr: true,
		},
		{
			name:    "pull without version",
			ref:     "oci://registry.example.com/charts/mychart",
			version: "",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					mock.MatchedBy(func(args []string) bool {
						return args[0] == "pull" &&
							!containsArg(args, "--version")
					})).
					Return("", "", nil)
			},
			wantErr: true, // no .tgz created
		},
		{
			name:    "pull fails",
			ref:     "myrepo/mychart",
			version: "1.0.0",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					mock.MatchedBy(func(args []string) bool {
						return args[0] == "pull" && args[1] == "myrepo/mychart"
					})).
					Return("", "chart not found", assert.AnError)
			},
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

			path, err := client.PullByRef(t.Context(), tt.ref, tt.version)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, path)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, path)
			mockExec.AssertExpectations(t)
		})
	}
}

func TestHelmClient_PullByRef_SuccessCleansTempDir(t *testing.T) {
	mockExec := &MockBinaryExecutor{}

	var destinationDir string
	mockExec.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
		mock.MatchedBy(func(args []string) bool {
			return len(args) >= 2 && args[0] == "pull" && args[1] == "myrepo/mychart" && containsArg(args, "--destination")
		})).
		Run(func(args mock.Arguments) {
			cmdArgs := args.Get(3).([]string)
			for i := 0; i < len(cmdArgs)-1; i++ {
				if cmdArgs[i] == "--destination" {
					destinationDir = cmdArgs[i+1]
					break
				}
			}
			require.NotEmpty(t, destinationDir)
			require.NoError(t, os.WriteFile(filepath.Join(destinationDir, "mychart-1.0.0.tgz"), []byte("chart"), 0o644))
		}).
		Return("", "", nil)

	client := &HelmClient{
		helmPath: "/usr/local/bin/helm",
		executor: mockExec,
	}

	path, err := client.PullByRef(t.Context(), "myrepo/mychart", "1.0.0")
	require.NoError(t, err)
	require.NotEmpty(t, path)
	defer os.Remove(path)

	_, statErr := os.Stat(destinationDir)
	require.True(t, os.IsNotExist(statErr), "temporary pull directory should be removed")
	mockExec.AssertExpectations(t)
}

func TestHelmClient_RegistryAuth(t *testing.T) {
	tests := []struct {
		name      string
		server    string
		user      string
		pass      string
		setupMock func(*MockBinaryExecutor)
		wantErr   bool
	}{
		{
			name:   "login with domain only",
			server: "registry.example.com",
			user:   "user",
			pass:   "pass",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommandWithInput", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					[]string{"registry", "login", "registry.example.com", "--username", "user", "--password-stdin"}).
					Return("", "", nil)
			},
		},
		{
			name:   "login strips https prefix",
			server: "https://registry.example.com",
			user:   "user",
			pass:   "pass",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommandWithInput", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					[]string{"registry", "login", "registry.example.com", "--username", "user", "--password-stdin"}).
					Return("", "", nil)
			},
		},
		{
			name:   "login strips http prefix",
			server: "http://registry.example.com",
			user:   "user",
			pass:   "pass",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommandWithInput", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					[]string{"registry", "login", "registry.example.com", "--username", "user", "--password-stdin"}).
					Return("", "", nil)
			},
		},
		{
			name:   "login fails",
			server: "registry.example.com",
			user:   "user",
			pass:   "badpass",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommandWithInput", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					[]string{"registry", "login", "registry.example.com", "--username", "user", "--password-stdin"}).
					Return("", "unauthorized", assert.AnError)
			},
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

			err := client.RegistryAuth(t.Context(), tt.server, tt.user, tt.pass)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			mockExec.AssertExpectations(t)
		})
	}
}

func TestHelmClient_Push(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		dst       string
		setupMock func(*MockBinaryExecutor)
		wantErr   bool
	}{
		{
			name: "successful push",
			path: "/tmp/mychart-1.0.0.tgz",
			dst:  "oci://registry.example.com/charts",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					[]string{"push", "/tmp/mychart-1.0.0.tgz", "oci://registry.example.com/charts"}).
					Return("", "", nil)
			},
		},
		{
			name: "push fails",
			path: "/tmp/mychart-1.0.0.tgz",
			dst:  "oci://registry.example.com/charts",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					[]string{"push", "/tmp/mychart-1.0.0.tgz", "oci://registry.example.com/charts"}).
					Return("", "unauthorized", assert.AnError)
			},
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

			err := client.Push(t.Context(), tt.path, tt.dst)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			mockExec.AssertExpectations(t)
		})
	}
}

func Test_cleanUpGenericMap(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]any
		want map[string]any
	}{
		{
			name: "single level map",
			in: map[string]any{
				"abc":    "xyz",
				"number": 5,
				"float":  1.5,
				"bool":   true,
				"array": []any{
					"element",
				},
			},
			want: map[string]any{
				"abc":    "xyz",
				"number": float64(5),
				"float":  1.5,
				"bool":   true,
				"array": []any{
					"element",
				},
			},
		},
		{
			name: "nested map, string keys",
			in: map[string]any{
				"nest": map[string]any{
					"abc":    "xyz",
					"number": 5,
					"float":  1.5,
					"bool":   true,
					"array": []any{
						"element",
					},
				},
			},
			want: map[string]any{
				"nest": map[string]any{
					"abc":    "xyz",
					"number": float64(5),
					"float":  1.5,
					"bool":   true,
					"array": []any{
						"element",
					},
				},
			},
		},
		{
			name: "nested map, interface keys", // this is what would fail previously
			in: map[string]any{
				"nest": map[any]any{
					"abc":    "xyz",
					"number": 5,
					"float":  1.5,
					"bool":   true,
					"array": []any{
						"element",
					},
				},
			},
			want: map[string]any{
				"nest": map[string]any{
					"abc":    "xyz",
					"number": float64(5),
					"float":  1.5,
					"bool":   true,
					"array": []any{
						"element",
					},
				},
			},
		},
		{
			name: "nested map, generic map array keys",
			in: map[string]any{
				"nest": map[any]any{
					"abc":    "xyz",
					"number": 5,
					"float":  1.5,
					"bool":   true,
					"array": []map[string]any{
						{
							"name":  "example",
							"value": "true",
						},
					},
				},
			},
			want: map[string]any{
				"nest": map[string]any{
					"abc":    "xyz",
					"number": float64(5),
					"float":  1.5,
					"bool":   true,
					"array": []any{
						map[string]any{
							"name":  "example",
							"value": "true",
						},
					},
				},
			},
		},
		{
			name: "nested map, interface map array keys",
			in: map[string]any{
				"nest": map[any]any{
					"abc":    "xyz",
					"number": 5,
					"float":  1.5,
					"bool":   true,
					"array": []map[any]any{
						{
							"name":  "example",
							"value": "true",
						},
					},
				},
			},
			want: map[string]any{
				"nest": map[string]any{
					"abc":    "xyz",
					"number": float64(5),
					"float":  1.5,
					"bool":   true,
					"array": []any{
						map[string]any{
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

// containsArg checks if slice contains element
func containsArg(slice []string, elem string) bool {
	for _, s := range slice {
		if s == elem {
			return true
		}
	}
	return false
}

// containsArgPair checks if slice contains consecutive pair key, value
func containsArgPair(slice []string, key, value string) bool {
	for i := 0; i < len(slice)-1; i++ {
		if slice[i] == key && slice[i+1] == value {
			return true
		}
	}
	return false
}
