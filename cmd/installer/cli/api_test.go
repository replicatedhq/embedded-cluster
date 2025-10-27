package cli

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"testing/fstest"
	"time"

	apilogger "github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg-new/tlsutils"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/web"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	helmcli "helm.sh/helm/v3/pkg/cli"
	"k8s.io/apimachinery/pkg/version"
)

func Test_serveAPI(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)

	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = listener.Close()
	})

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	errCh := make(chan error)

	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)

	cert, _, _, err := tlsutils.GenerateCertificate("localhost", nil)
	require.NoError(t, err)

	certPool := x509.NewCertPool()
	certPool.AddCert(cert.Leaf)

	// Mock the web assets filesystem so that we don't need to embed the web assets.
	webAssetsFS = fstest.MapFS{
		"index.html": &fstest.MapFile{
			Data: []byte(""),
			Mode: 0644,
		},
	}

	portInt, err := strconv.Atoi(port)
	require.NoError(t, err)

	password := "password"
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	require.NoError(t, err)

	rc := setupMockRuntimeConfig(t)

	config := apiOptions{
		APIConfig: apitypes.APIConfig{
			InstallTarget: apitypes.InstallTargetLinux,
			Password:      password,
			PasswordHash:  passwordHash,
			ReleaseData: &release.ReleaseData{
				Application: &kotsv1beta1.Application{
					Spec: kotsv1beta1.ApplicationSpec{
						Title: "Test Application",
					},
				},
				AppConfig: &kotsv1beta1.Config{
					Spec: kotsv1beta1.ConfigSpec{},
				},
			},
			ClusterID: "123",
			Mode:      apitypes.ModeInstall,
			LinuxConfig: apitypes.LinuxConfig{
				RuntimeConfig: rc,
			},
		},
		ManagerPort: portInt,
		WebMode:     web.ModeInstall,
		Logger:      apilogger.NewDiscardLogger(),
		WebAssetsFS: webAssetsFS,
	}

	go func() {
		err := serveAPI(ctx, listener, cert, config)
		t.Logf("Install API exited with error: %v", err)
		errCh <- err
	}()

	url := "https://" + net.JoinHostPort("localhost", port) + "/api/health"
	t.Logf("Making request to %s", url)
	httpClient := http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certPool,
			},
		},
	}
	resp, err := httpClient.Get(url)
	require.NoError(t, err)
	if resp != nil {
		defer resp.Body.Close()
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	cancel()
	assert.ErrorIs(t, <-errCh, http.ErrServerClosed)
	t.Logf("Install API exited")
}

func Test_serveAPIHTMLInjection(t *testing.T) {
	tests := []struct {
		name          string
		installTarget apitypes.InstallTarget
		mode          web.Mode
		title         string
	}{
		{"linux install mode", apitypes.InstallTargetLinux, web.ModeInstall, "Linux Install App"},
		{"linux upgrade mode", apitypes.InstallTargetLinux, web.ModeUpgrade, "Linux Upgrade App"},
		{"kubernetes install mode", apitypes.InstallTargetKubernetes, web.ModeInstall, "K8s Install App"},
		{"kubernetes upgrade mode", apitypes.InstallTargetKubernetes, web.ModeUpgrade, "K8s Upgrade App"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logrus.SetLevel(logrus.DebugLevel)

			listener, err := net.Listen("tcp", ":0")
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = listener.Close()
			})

			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)

			errCh := make(chan error)

			_, port, err := net.SplitHostPort(listener.Addr().String())
			require.NoError(t, err)

			cert, _, _, err := tlsutils.GenerateCertificate("localhost", nil)
			require.NoError(t, err)

			certPool := x509.NewCertPool()
			certPool.AddCert(cert.Leaf)

			// Mock the web assets filesystem
			webAssetsFS = fstest.MapFS{
				"index.html": &fstest.MapFile{
					Data: []byte(`<!DOCTYPE html><html><head><title>{{.Title}}</title></head><body><script>window.initialState = {{.InitialState}};</script></body></html>`),
					Mode: 0644,
				},
			}

			portInt, err := strconv.Atoi(port)
			require.NoError(t, err)

			password := "password"
			passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
			require.NoError(t, err)

			config := apiOptions{
				APIConfig: apitypes.APIConfig{
					InstallTarget: tt.installTarget,
					Password:      password,
					PasswordHash:  passwordHash,
					ReleaseData: &release.ReleaseData{
						Application: &kotsv1beta1.Application{
							Spec: kotsv1beta1.ApplicationSpec{
								Title: tt.title,
							},
						},
						AppConfig: &kotsv1beta1.Config{
							Spec: kotsv1beta1.ConfigSpec{},
						},
					},
					ClusterID: "123",
					Mode:      apitypes.ModeInstall,
				},
				ManagerPort: portInt,
				WebMode:     tt.mode,
				Logger:      apilogger.NewDiscardLogger(),
				WebAssetsFS: webAssetsFS,
			}

			if tt.installTarget == apitypes.InstallTargetKubernetes {
				ki := setupMockKubernetesInstallation(t)
				config.Installation = ki
			} else {
				// Create a runtime config with temp directory
				rc := setupMockRuntimeConfig(t)
				config.RuntimeConfig = rc
			}

			go func() {
				err := serveAPI(ctx, listener, cert, config)
				t.Logf("Install API exited with error: %v", err)
				errCh <- err
			}()

			httpClient := http.Client{
				Timeout: 2 * time.Second,
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						RootCAs: certPool,
					},
				},
			}

			// Test that the root HTML page contains the correct mode and install target
			rootURL := "https://" + net.JoinHostPort("localhost", port) + "/"
			rootResp, err := httpClient.Get(rootURL)
			require.NoError(t, err)
			defer rootResp.Body.Close()

			assert.Equal(t, http.StatusOK, rootResp.StatusCode)

			// Read the HTML response body
			body, err := io.ReadAll(rootResp.Body)
			require.NoError(t, err)
			htmlStr := string(body)

			// Verify the HTML contains the marshaled initial state with correct values
			assert.Contains(t, htmlStr, fmt.Sprintf(`"mode":"%s"`, tt.mode))
			assert.Contains(t, htmlStr, fmt.Sprintf(`"installTarget":"%s"`, tt.installTarget))
			assert.Contains(t, htmlStr, fmt.Sprintf(`"title":"%s"`, tt.title))

			cancel()
			assert.ErrorIs(t, <-errCh, http.ErrServerClosed)
			t.Logf("Install API exited")
		})
	}
}

func setupMockRuntimeConfig(t *testing.T) *runtimeconfig.MockRuntimeConfig {
	// Set up mock Kubernetes API server for helm client to use
	mockK8sServer := setupMockKubernetesAPI(t)
	t.Cleanup(func() {
		mockK8sServer.Close()
	})
	t.Setenv("HELM_KUBEAPISERVER", mockK8sServer.URL)
	fmt.Println("HELM_KUBEAPISERVER", mockK8sServer.URL)

	// Write the helm binary to the temp directory for helm client to use
	helmPath := filepath.Join(t.TempDir(), "helm")
	err := helpers.WriteFile(helmPath, []byte(mockK8sServer.URL), 0644)
	require.NoError(t, err)

	rc := &runtimeconfig.MockRuntimeConfig{}
	rc.On("GetKubernetesEnvSettings").Return(helmcli.New())
	rc.On("PathToEmbeddedClusterBinary", "helm").Return(helmPath, nil)
	return rc
}

func setupMockKubernetesInstallation(t *testing.T) *kubernetesinstallation.MockInstallation {
	// Set up mock Kubernetes API server for helm client to use
	mockK8sServer := setupMockKubernetesAPI(t)
	t.Cleanup(func() {
		mockK8sServer.Close()
	})
	t.Setenv("HELM_KUBEAPISERVER", mockK8sServer.URL)
	fmt.Println("HELM_KUBEAPISERVER", mockK8sServer.URL)

	// Write the helm binary to the temp directory for helm client to use
	helmPath := filepath.Join(t.TempDir(), "helm")
	err := helpers.WriteFile(helmPath, []byte(mockK8sServer.URL), 0644)
	require.NoError(t, err)

	ki := &kubernetesinstallation.MockInstallation{}
	ki.On("GetKubernetesEnvSettings").Return(helmcli.New())
	ki.On("PathToEmbeddedBinary", "helm").Return(helmPath, nil)
	return ki
}

// setupMockKubernetesAPI creates a mock Kubernetes API server for testing
func setupMockKubernetesAPI(_ *testing.T) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/version":
			// Return a mock Kubernetes version
			versionInfo := version.Info{
				Major:      "1",
				Minor:      "28",
				GitVersion: "v1.28.0",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(versionInfo)
		default:
			// Return 404 for other endpoints
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	return server
}
