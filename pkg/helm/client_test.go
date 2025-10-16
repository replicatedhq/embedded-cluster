package helm

import (
	"testing"

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
