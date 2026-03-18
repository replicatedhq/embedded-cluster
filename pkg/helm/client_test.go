package helm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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

func Test_parseReleaseOutput(t *testing.T) {
	input := `{
		"name": "myrelease",
		"namespace": "default",
		"version": 3,
		"info": {"status": "deployed"},
		"chart": {"metadata": {"name": "mychart", "version": "1.2.3"}}
	}`
	got, err := parseReleaseOutput(input)
	require.NoError(t, err)
	assert.Equal(t, &ReleaseInfo{
		Name:      "myrelease",
		Namespace: "default",
		Status:    "deployed",
		Revision:  3,
		Chart:     "mychart",
		Version:   "1.2.3",
	}, got)
}

func Test_parseReleaseOutput_invalidJSON(t *testing.T) {
	_, err := parseReleaseOutput("not json")
	assert.Error(t, err)
}

func TestHelmClient_writeValuesToTemp(t *testing.T) {
	t.Run("empty values returns empty path", func(t *testing.T) {
		client := &HelmClient{tmpdir: t.TempDir()}
		path, err := client.writeValuesToTemp(nil)
		require.NoError(t, err)
		assert.Empty(t, path)
	})

	t.Run("non-empty values writes YAML file", func(t *testing.T) {
		client := &HelmClient{tmpdir: t.TempDir()}
		path, err := client.writeValuesToTemp(map[string]interface{}{"key": "value"})
		require.NoError(t, err)
		assert.NotEmpty(t, path)
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(data), "key: value")
	})
}

const testReleaseJSON = `{
	"name": "myrelease",
	"namespace": "default",
	"version": 1,
	"info": {"status": "deployed"},
	"chart": {"metadata": {"name": "mychart", "version": "1.0.0"}}
}`

func TestHelmClient_Install(t *testing.T) {
	tests := []struct {
		name      string
		opts      func(chartFile string) InstallOptions
		setupMock func(*MockBinaryExecutor, string)
		want      *ReleaseInfo
		wantErr   bool
	}{
		{
			name: "basic install",
			opts: func(chartFile string) InstallOptions {
				return InstallOptions{
					ReleaseName: "myrelease",
					ChartPath:   chartFile,
					Namespace:   "default",
					Timeout:     5 * time.Minute,
				}
			},
			setupMock: func(m *MockBinaryExecutor, chartFile string) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					mock.MatchedBy(func(args []string) bool {
						return args[0] == "install" &&
							args[1] == "myrelease" &&
							args[2] == chartFile &&
							slicesContains(args, "--namespace") &&
							slicesContains(args, "default") &&
							slicesContains(args, "--create-namespace") &&
							slicesContains(args, "--wait") &&
							slicesContains(args, "--output") &&
							slicesContains(args, "json") &&
							!slicesContains(args, "--atomic") // Helm 4 does not use --atomic on install
					}),
				).Return(testReleaseJSON, "", nil)
			},
			want: &ReleaseInfo{
				Name: "myrelease", Namespace: "default",
				Status: "deployed", Revision: 1,
				Chart: "mychart", Version: "1.0.0",
			},
		},
		{
			name: "helm install fails",
			opts: func(chartFile string) InstallOptions {
				return InstallOptions{ReleaseName: "bad", ChartPath: chartFile, Namespace: "ns"}
			},
			wantErr: true,
			setupMock: func(m *MockBinaryExecutor, chartFile string) {
				m.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
					mock.MatchedBy(func(args []string) bool { return args[0] == "install" }),
				).Return("", "Error: chart not found", assert.AnError)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a real temp chart file so resolveChartPath's os.Stat succeeds
			chartFile := filepath.Join(t.TempDir(), "mychart-1.0.0.tgz")
			require.NoError(t, os.WriteFile(chartFile, []byte("fake"), 0644))

			mockExec := &MockBinaryExecutor{}
			tt.setupMock(mockExec, chartFile)
			client := &HelmClient{executor: mockExec, tmpdir: t.TempDir()}
			got, err := client.Install(t.Context(), tt.opts(chartFile))
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

// slicesContains is a test helper that checks if a string slice contains a value.
func slicesContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func TestHelmClient_Upgrade(t *testing.T) {
	tests := []struct {
		name          string
		opts          UpgradeOptions
		forbiddenArgs []string
		requiredArgs  []string
		wantErr       bool
	}{
		{
			name: "upgrade always uses --rollback-on-failure not --atomic",
			opts: UpgradeOptions{ReleaseName: "r", ChartPath: "", Namespace: "ns"},
			requiredArgs:  []string{"upgrade", "--rollback-on-failure", "--output", "json"},
			forbiddenArgs: []string{"--atomic", "--force", "--force-replace"},
		},
		{
			name: "Force=true maps to --force-replace not --force",
			opts: UpgradeOptions{ReleaseName: "r", ChartPath: "", Namespace: "ns", Force: true},
			requiredArgs:  []string{"upgrade", "--rollback-on-failure", "--force-replace", "--output", "json"},
			forbiddenArgs: []string{"--atomic", "--force"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a real temp file so resolveChartPath succeeds
			chartFile := filepath.Join(t.TempDir(), "mychart-1.0.0.tgz")
			require.NoError(t, os.WriteFile(chartFile, []byte("fake"), 0644))
			tt.opts.ChartPath = chartFile

			mockExec := &MockBinaryExecutor{}
			mockExec.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
				mock.MatchedBy(func(args []string) bool {
					for _, required := range tt.requiredArgs {
						if !slicesContains(args, required) {
							return false
						}
					}
					for _, forbidden := range tt.forbiddenArgs {
						if slicesContains(args, forbidden) {
							return false
						}
					}
					return true
				}),
			).Return(testReleaseJSON, "", nil)

			client := &HelmClient{executor: mockExec, tmpdir: t.TempDir()}
			_, err := client.Upgrade(t.Context(), tt.opts)
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
		name         string
		opts         UninstallOptions
		requiredArgs []string
	}{
		{
			name: "basic uninstall",
			opts: UninstallOptions{ReleaseName: "myrelease", Namespace: "default"},
			requiredArgs: []string{"uninstall", "myrelease", "--namespace", "default"},
		},
		{
			name: "with wait and ignore-not-found",
			opts: UninstallOptions{ReleaseName: "r", Namespace: "ns", Wait: true, IgnoreNotFound: true},
			requiredArgs: []string{"uninstall", "r", "--namespace", "ns", "--wait", "--ignore-not-found"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := &MockBinaryExecutor{}
			mockExec.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
				mock.MatchedBy(func(args []string) bool {
					for _, r := range tt.requiredArgs {
						if !slicesContains(args, r) {
							return false
						}
					}
					return true
				}),
			).Return("", "", nil)
			client := &HelmClient{executor: mockExec, tmpdir: t.TempDir()}
			err := client.Uninstall(t.Context(), tt.opts)
			require.NoError(t, err)
			mockExec.AssertExpectations(t)
		})
	}
}

func TestHelmClient_Render(t *testing.T) {
	v, _ := semver.NewVersion("1.29.0")
	mockOut := "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n---\napiVersion: v1\nkind: Service\nmetadata:\n  name: svc\n"

	// Create a real chart file so resolveChartPath succeeds
	chartFile := filepath.Join(t.TempDir(), "mychart-1.0.0.tgz")
	require.NoError(t, os.WriteFile(chartFile, []byte("fake"), 0644))

	mockExec := &MockBinaryExecutor{}
	mockExec.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
		mock.MatchedBy(func(args []string) bool {
			return args[0] == "template" &&
				slicesContains(args, "--include-crds") &&
				slicesContains(args, "--kube-version") &&
				!slicesContains(args, "--dry-run") // helm template doesn't need --dry-run
		}),
	).Return(mockOut, "", nil)

	client := &HelmClient{executor: mockExec, tmpdir: t.TempDir(), kversion: v}
	manifests, err := client.Render(t.Context(), InstallOptions{
		ReleaseName: "r",
		ChartPath:   chartFile,
		Namespace:   "ns",
	})
	require.NoError(t, err)
	assert.Len(t, manifests, 2)
	mockExec.AssertExpectations(t)
}

func TestHelmClient_PullByRef(t *testing.T) {
	t.Run("pulls chart to temp dir", func(t *testing.T) {
		mockExec := &MockBinaryExecutor{}
		tmpdir := t.TempDir()

		mockExec.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
			mock.MatchedBy(func(args []string) bool {
				return args[0] == "pull" &&
					args[1] == "oci://registry.example.com/mychart" &&
					slicesContains(args, "--version") && slicesContains(args, "1.0.0") &&
					slicesContains(args, "--destination")
			}),
		).Run(func(args mock.Arguments) {
			helmArgs := args.Get(3).([]string)
			for i, a := range helmArgs {
				if a == "--destination" && i+1 < len(helmArgs) {
					destDir := helmArgs[i+1]
					os.MkdirAll(destDir, 0755)
					os.WriteFile(filepath.Join(destDir, "mychart-1.0.0.tgz"), []byte("fake"), 0644)
				}
			}
		}).Return("", "", nil)

		client := &HelmClient{executor: mockExec, tmpdir: tmpdir}
		path, err := client.PullByRef(t.Context(), "oci://registry.example.com/mychart", "1.0.0")
		require.NoError(t, err)
		assert.True(t, strings.HasSuffix(path, ".tgz"))
		mockExec.AssertExpectations(t)
	})
}

func TestHelmClient_Push(t *testing.T) {
	mockExec := &MockBinaryExecutor{}
	mockExec.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
		mock.MatchedBy(func(args []string) bool {
			return args[0] == "push" &&
				args[1] == "/tmp/mychart-1.0.0.tgz" &&
				args[2] == "oci://registry.example.com"
		}),
	).Return("", "", nil)

	client := &HelmClient{executor: mockExec, tmpdir: t.TempDir()}
	err := client.Push(t.Context(), "/tmp/mychart-1.0.0.tgz", "oci://registry.example.com")
	require.NoError(t, err)
	mockExec.AssertExpectations(t)
}

func TestHelmClient_RegistryAuth_StripsScheme(t *testing.T) {
	tests := []struct {
		server     string
		wantDomain string
	}{
		{"https://registry.example.com", "registry.example.com"},
		{"http://registry.example.com", "registry.example.com"},
		{"registry.example.com", "registry.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.server, func(t *testing.T) {
			mockExec := &MockBinaryExecutor{}
			mockExec.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
				mock.MatchedBy(func(args []string) bool {
					return args[0] == "registry" && args[1] == "login" && args[2] == tt.wantDomain
				}),
			).Return("", "", nil)

			client := &HelmClient{executor: mockExec, tmpdir: t.TempDir()}
			err := client.RegistryAuth(t.Context(), tt.server, "user", "pass")
			require.NoError(t, err)
			mockExec.AssertExpectations(t)
		})
	}
}

func TestHelmClient_ReleaseExists(t *testing.T) {
	tests := []struct {
		name    string
		release string
		mockOut string
		want    bool
		wantErr bool
	}{
		{
			name:    "deployed release exists",
			release: "myrelease",
			mockOut: `[{"name":"myrelease","namespace":"default","status":"deployed","revision":"1","chart":"mychart-1.0.0"}]`,
			want:    true,
		},
		{
			name:    "empty list means not found",
			release: "notexist",
			mockOut: `[]`,
			want:    false,
		},
		{
			name:    "uninstalling treated as not found",
			release: "myrelease",
			mockOut: `[{"name":"myrelease","namespace":"default","status":"uninstalling","revision":"1","chart":"c-1.0.0"}]`,
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := &MockBinaryExecutor{}
			mockExec.On("ExecuteCommand", mock.Anything, mock.Anything, mock.Anything,
				mock.MatchedBy(func(args []string) bool {
					return args[0] == "list" &&
						slicesContains(args, "--output") && slicesContains(args, "json") &&
						slicesContains(args, "--all")
				}),
			).Return(tt.mockOut, "", nil)

			client := &HelmClient{executor: mockExec, tmpdir: t.TempDir()}
			got, err := client.ReleaseExists(t.Context(), "default", tt.release)
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
