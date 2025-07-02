package cli

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"strconv"
	"testing"
	"testing/fstest"
	"time"

	apilogger "github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/tlsutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	config := apiOptions{
		APIConfig: apitypes.APIConfig{
			Password: "password",
			ReleaseData: &release.ReleaseData{
				Application: &kotsv1beta1.Application{
					Spec: kotsv1beta1.ApplicationSpec{},
				},
			},
			ClusterID: "123",
		},
		ManagerPort: portInt,
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
