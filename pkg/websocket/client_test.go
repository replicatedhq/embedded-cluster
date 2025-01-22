package websocket

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	gwebsocket "github.com/gorilla/websocket"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers/systemd"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/websocket/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUpgradeManagerCommand(t *testing.T) {
	runtimeconfig.SetDataDir(t.TempDir())

	// Create and start test server
	ts := NewTestServer(t)
	defer ts.Close()

	// Set KOTS port from test server
	t.Setenv("KOTS_PORT", fmt.Sprintf("%d", ts.Server.Listener.Addr().(*net.TCPAddr).Port))

	// Create mock systemd
	mockDBus := &systemd.MockDBus{}
	systemd.Set(mockDBus)
	defer systemd.Set(&systemd.Systemd{})

	// Setup expectations
	mockDBus.On("Restart", mock.Anything, "test-app-manager.service").Return(nil)

	// Create fake k8s client
	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	assert.NoError(t, err)

	kotsService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kotsadm",
			Namespace: runtimeconfig.KotsadmNamespace,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: ts.Server.Listener.Addr().(*net.TCPAddr).IP.String(),
		},
	}

	hostname, err := os.Hostname()
	assert.NoError(t, err)

	testNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: hostname,
		},
	}

	kcli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(kotsService, testNode).
		Build()

	// Start websocket client
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go ConnectToKOTSWebSocket(ctx, kcli)

	// Wait for websocket connection to be established
	ts.WaitForConn()

	// Send upgrade-manager command
	msg := types.Message{
		AppSlug:      "test-app",
		VersionLabel: "v1.0.0",
		StepID:       "test-step",
		Command:      types.CommandUpgradeManager,
		Data: string(mustMarshal(t, types.UpgradeManagerData{
			LicenseID:       "test-license",
			LicenseEndpoint: ts.Server.URL,
		})),
	}
	err = ts.GetWSConn().WriteJSON(msg)
	assert.NoError(t, err)

	// Wait for and verify report
	assert.Eventually(t, func() bool {
		return len(ts.ReportCalls) > 0
	}, time.Second*5, time.Millisecond*100, "Did not receive report")
	assert.Equal(t, "running", ts.ReportCalls[0].Status)
	assert.Equal(t, "", ts.ReportCalls[0].Desc)

	// Verify binary contents
	downloadedBinary := filepath.Join(runtimeconfig.EmbeddedClusterBinsSubDir(), "manager")
	gotBinContent, err := os.ReadFile(downloadedBinary)
	assert.NoError(t, err)
	assert.Equal(t, "TESTING", string(gotBinContent))

	// Verify binary file permissions
	binInfo, err := os.Stat(downloadedBinary)
	assert.NoError(t, err)
	assert.Equal(t, "-rw-r--r--", binInfo.Mode().String())

	// Verify systemd calls
	mockDBus.AssertExpectations(t)
}

func mustMarshal(t *testing.T, v interface{}) []byte {
	data, err := json.Marshal(v)
	assert.NoError(t, err)
	return data
}

// TestServer is a mock KOTS admin console for testing
type TestServer struct {
	Server      *httptest.Server
	ReportCalls []struct {
		Status string
		Desc   string
	}
	WSConn *gwebsocket.Conn
	t      *testing.T
}

// NewTestServer creates a new test server with all the required endpoints
func NewTestServer(t *testing.T) *TestServer {
	ts := &TestServer{
		t: t,
	}

	// Create the test server
	ts.Server = httptest.NewServer(http.HandlerFunc(ts.handler))

	return ts
}

func (ts *TestServer) handler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/clusterconfig/artifact/manager":
		// Verify basic auth
		username, password, ok := r.BasicAuth()
		if !ok || username != "test-license" || password != "test-license" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Create dummy binary content
		testDir := ts.t.TempDir()
		testBinFile := filepath.Join(testDir, "manager")
		if err := os.WriteFile(testBinFile, []byte("TESTING"), 0644); err != nil {
			ts.t.Fatalf("Failed to write test file: %v", err)
		}
		testBinTGZ := filepath.Join(testDir, "manager.tar.gz")
		if err := createTestTarGz(testBinTGZ, testBinFile, "manager"); err != nil {
			ts.t.Fatalf("Failed to create test tar.gz: %v", err)
		}

		// Serve the binary
		http.ServeFile(w, r, testBinTGZ)

	case "/ec-ws":
		// Handle websocket connection
		var err error
		upgrader := gwebsocket.Upgrader{}
		ts.WSConn, err = upgrader.Upgrade(w, r, nil)
		if err != nil {
			ts.t.Errorf("Failed to upgrade connection: %v", err)
			return
		}

	case "/api/v1/app/test-app/plan/test-step":
		// Track report calls
		var report struct {
			Status            string `json:"status"`
			StatusDescription string `json:"statusDescription"`
		}
		if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
			ts.t.Errorf("Failed to decode report: %v", err)
			return
		}
		ts.ReportCalls = append(ts.ReportCalls, struct {
			Status string
			Desc   string
		}{
			Status: report.Status,
			Desc:   report.StatusDescription,
		})
		w.WriteHeader(http.StatusOK)

	default:
		http.NotFound(w, r)
	}
}

// WaitForConn waits for the websocket connection to be established
func (ts *TestServer) WaitForConn() {
	assert.Eventually(ts.t, func() bool {
		return ts.GetWSConn() != nil
	}, time.Second*5, time.Millisecond*100, "Websocket connection should be established")
}

// Close shuts down the test server
func (ts *TestServer) Close() {
	ts.Server.Close()
}

// GetWSConn returns the websocket connection once established
func (ts *TestServer) GetWSConn() *gwebsocket.Conn {
	return ts.WSConn
}

func createTestTarGz(tarGzPath, srcPath, tarPath string) error {
	tarGzFile, err := os.Create(tarGzPath)
	if err != nil {
		return err
	}
	defer tarGzFile.Close()

	gzWriter := gzip.NewWriter(tarGzFile)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	file, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name:    tarPath,
		Size:    stat.Size(),
		Mode:    int64(stat.Mode()),
		ModTime: stat.ModTime(),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	if _, err := io.Copy(tarWriter, file); err != nil {
		return err
	}

	return nil
}
