package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeCmd(t *testing.T) {
	// Create temporary directory for test
	dataDir := t.TempDir()
	t.Setenv("TMPDIR", dataDir) // hack as the cli sets TMPDIR, this will reset it after the test

	rc := runtimeconfig.New(nil)
	rc.SetDataDir(dataDir)

	// Detect a free port
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	listener.Close()
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)

	// Create the required bin/local-artifact-mirror binary
	binPath := filepath.Join(dataDir, "bin", "local-artifact-mirror")
	err = os.MkdirAll(filepath.Dir(binPath), 0755)
	require.NoError(t, err)
	err = os.WriteFile(binPath, []byte("test content"), 0644)
	require.NoError(t, err)

	// Create test files in whitelisted directories
	for _, dir := range whitelistServeDirs {
		dirPath := filepath.Join(dataDir, dir)
		err := os.MkdirAll(dirPath, 0755)
		require.NoError(t, err)

		testFilePath := filepath.Join(dirPath, "test.txt")
		err = os.WriteFile(testFilePath, []byte("test content"), 0644)
		require.NoError(t, err)
	}

	// Create a test file in a non-whitelisted directory
	nonWhitelistedDir := filepath.Join(dataDir, "logs")
	err = os.MkdirAll(nonWhitelistedDir, 0755)
	require.NoError(t, err)

	nonWhitelistedFile := filepath.Join(nonWhitelistedDir, "secret.txt")
	err = os.WriteFile(nonWhitelistedFile, []byte("secret content"), 0644)
	require.NoError(t, err)

	testCases := []struct {
		name       string
		setupEnv   func(t *testing.T)
		args       []string
		filePath   string
		expectCode int
		expectBody string
	}{
		{
			name:       "access whitelisted file with flags",
			args:       []string{"--port", port, "--data-dir", dataDir},
			filePath:   "/bin/test.txt",
			expectCode: http.StatusOK,
			expectBody: "test content",
		},
		{
			name: "access whitelisted file with env var",
			setupEnv: func(t *testing.T) {
				// Set environment variables
				t.Setenv("LOCAL_ARTIFACT_MIRROR_PORT", port)
				t.Setenv("LOCAL_ARTIFACT_MIRROR_DATA_DIR", dataDir)
			},
			filePath:   "/charts/test.txt",
			expectCode: http.StatusOK,
			expectBody: "test content",
		},
		{
			name: "cannot access non-whitelisted file",
			setupEnv: func(t *testing.T) {
				// Set environment variables
				t.Setenv("LOCAL_ARTIFACT_MIRROR_PORT", port)
				t.Setenv("LOCAL_ARTIFACT_MIRROR_DATA_DIR", dataDir)
			},
			filePath:   "/logs/secret.txt",
			expectCode: http.StatusNotFound,
			expectBody: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup environment for test
			if tc.setupEnv != nil {
				tc.setupEnv(t)
			}

			// Setup the commands
			cli := &CLI{
				RC:   rc,
				Name: "local-artifact-mirror",
				V:    viper.New(),
			}
			root := RootCmd(cli)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			// Execute command in a goroutine
			errCh := make(chan error, 1)
			go func() {
				_, _, err := testExecuteCommandC(ctx, root, append([]string{"serve"}, tc.args...)...)
				errCh <- err
			}()

			// Wait for server to start
			for range 10 {
				time.Sleep(1 * time.Second)
				resp, err := http.Get("http://127.0.0.1:" + port)
				if err == nil {
					resp.Body.Close()
					break
				}
			}

			// Make request to server
			url := "http://127.0.0.1:" + port + tc.filePath
			resp, err := http.Get(url)

			switch tc.expectCode {
			case http.StatusOK:
				require.NoError(t, err, "HTTP request should not fail")
				defer resp.Body.Close()

				assert.Equal(t, tc.expectCode, resp.StatusCode)

				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				assert.Equal(t, tc.expectBody, string(body))
			case http.StatusNotFound:
				if err == nil {
					defer resp.Body.Close()
					assert.Equal(t, tc.expectCode, resp.StatusCode)
				} else {
					// In case the request returns a connection error due to 404
					// This is also acceptable
					t.Logf("Expected error: %v", err)
				}
			}

			// Cancel context to stop server
			cancel()

			// Wait for command to finish
			select {
			case err := <-errCh:
				assert.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("Command did not exit in time")
			}
		})
	}
}
