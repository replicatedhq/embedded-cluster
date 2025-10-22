package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	testBinaryContent = []byte(`#!/bin/bash
while [[ $# -gt 0 ]]; do
  case $1 in
    '--data-dir')
       shift
       echo "--data-dir = $1"
       echo "Hello, world!" > "$1/some-file"
       exit 0
       ;;
  esac
  shift
done
echo "did not find --data-dir"
exit 1
`)
)

func TestPullBinariesCmd_Online(t *testing.T) {
	// Create temporary directory for test
	dataDir := t.TempDir()
	t.Setenv("TMPDIR", dataDir) // hack as the cli sets TMPDIR, this will reset it after the test

	rc := runtimeconfig.New(nil)
	rc.SetDataDir(dataDir)

	// Create a test server that serves the test release archive
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check basic auth
		username, _, ok := r.BasicAuth()
		if !ok || username != "valid-license" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Set headers
		w.Header().Set("Content-Disposition", "attachment; filename=my-app.tgz")
		w.Header().Set("Content-Type", "application/gzip")
		w.WriteHeader(http.StatusOK)

		// Create a tar.gz archive with the test binary
		var buf bytes.Buffer
		gzWriter := gzip.NewWriter(&buf)
		tarWriter := tar.NewWriter(gzWriter)

		// Add the test binary to the archive with the expected name
		header := &tar.Header{
			Name: "my-app",
			Mode: 0755,
			Size: int64(len(testBinaryContent)),
		}
		err := tarWriter.WriteHeader(header)
		if err != nil {
			t.Fatalf("Failed to write tar header: %v", err)
		}

		_, err = tarWriter.Write(testBinaryContent)
		if err != nil {
			t.Fatalf("Failed to write tar content: %v", err)
		}

		// Close writers
		tarWriter.Close()
		gzWriter.Close()

		// Write the archive to the response
		w.Write(buf.Bytes())
	}))
	defer server.Close()

	// Create a fake client with test Installation
	scheme := runtime.NewScheme()
	err := ecv1beta1.AddToScheme(scheme)
	require.NoError(t, err)

	// Create a test installation
	installation := &ecv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-installation",
		},
		Spec: ecv1beta1.InstallationSpec{
			AirGap:         false,
			BinaryName:     "my-app",
			MetricsBaseURL: "https://api.replicated.com",
		},
	}

	installation.Spec.MetricsBaseURL = server.URL

	// Create fake client
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(installation).
		Build()

	testCases := []struct {
		name          string
		setupEnv      func(t *testing.T)
		args          []string
		mock          *mockPuller
		expectedError string
	}{
		{
			name: "successful online pull with license ID",
			args: []string{
				"test-installation",
				"--license-id", "valid-license",
				"--app-slug", "my-app",
				"--channel-id", "123",
				"--app-version", "1.0.0",
			},
			setupEnv: func(t *testing.T) {
				t.Setenv("LOCAL_ARTIFACT_MIRROR_DATA_DIR", dataDir)
			},
			mock: func() *mockPuller {
				m := &mockPuller{}
				// No need to mock PullArtifact for online mode
				return m
			}(),
			expectedError: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup environment for test
			if tc.setupEnv != nil {
				tc.setupEnv(t)
			}

			// Create the command
			cli := &CLI{
				RC:   rc,
				Name: "local-artifact-mirror",
				V:    viper.New(),
				KCLIGetter: func() (client.Client, error) {
					return fakeClient, nil
				},
				PullArtifact: tc.mock.PullArtifact,
			}
			root := RootCmd(cli)

			// Execute command
			_, _, err := testExecuteCommandC(t.Context(), root, append([]string{"pull", "binaries"}, tc.args...)...)

			tc.mock.AssertExpectations(t)

			if tc.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				require.NoError(t, err)

				// Check that the destination file exists
				expectedDst := filepath.Join(dataDir, "some-file")
				_, err := os.Stat(expectedDst)
				assert.NoError(t, err, "Expected destination file to exist")

				// Verify file content
				content, err := os.ReadFile(expectedDst)
				assert.NoError(t, err)
				assert.Equal(t, "Hello, world!\n", string(content))
			}
		})
	}
}
