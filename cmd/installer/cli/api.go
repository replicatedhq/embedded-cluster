package cli

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	apiclient "github.com/replicatedhq/embedded-cluster/api/client"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/tlsutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/web"
	"github.com/sirupsen/logrus"
)

// APIConfig holds the configuration for the API server
type APIConfig struct {
	Logger          logrus.FieldLogger
	MetricsReporter metrics.ReporterInterface
	Password        string
	ManagerPort     int
	IsAirgap        bool
	ConfigChan      chan<- *apitypes.InstallationConfig
}

func startAPI(ctx context.Context, cert tls.Certificate, config APIConfig) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", config.ManagerPort))
	if err != nil {
		return fmt.Errorf("unable to create listener: %w", err)
	}

	go func() {
		if err := serveAPI(ctx, listener, cert, config); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				logrus.Errorf("api error: %v", err)
			}
		}
	}()

	if err := waitForAPI(ctx, listener.Addr().String()); err != nil {
		return fmt.Errorf("unable to wait for api: %w", err)
	}

	return nil
}

func serveAPI(ctx context.Context, listener net.Listener, cert tls.Certificate, config APIConfig) error {
	router := mux.NewRouter()

	api, err := api.New(
		config.Password,
		api.WithLogger(config.Logger),
		api.WithMetricsReporter(config.MetricsReporter),
		api.WithConfigChan(config.ConfigChan),
		api.WithReleaseData(release.GetReleaseData()),
		api.WithIsAirgap(config.IsAirgap),
	)
	if err != nil {
		return fmt.Errorf("new api: %w", err)
	}

	api.RegisterRoutes(router.PathPrefix("/api").Subrouter())

	var webFs http.Handler
	if os.Getenv("EC_DEV_ENV") == "true" {
		webFs = http.FileServer(http.FS(os.DirFS("./web/dist")))
	} else {
		webFs = http.FileServer(http.FS(web.Fs()))
	}
	router.PathPrefix("/").Methods("GET").Handler(webFs)

	server := &http.Server{
		// ErrorLog outputs TLS errors and warnings to the console, we want to make sure we use the same logrus logger for them
		ErrorLog:  log.New(config.Logger.WithField("http-server", "std-log").Writer(), "", 0),
		Handler:   router,
		TLSConfig: tlsutils.GetTLSConfig(cert),
	}

	go func() {
		<-ctx.Done()
		logrus.Debugf("Shutting down API")
		server.Shutdown(context.Background())
	}()

	return server.ServeTLS(listener, "", "")
}

func waitForAPI(ctx context.Context, addr string) error {
	httpClient := http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			Proxy: nil,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	timeout := time.After(10 * time.Second)
	var lastErr error
	for {
		select {
		case <-timeout:
			if lastErr != nil {
				return fmt.Errorf("api did not start in time: %w", lastErr)
			}
			return fmt.Errorf("api did not start in time")
		case <-time.Tick(1 * time.Second):
			resp, err := httpClient.Get(fmt.Sprintf("https://%s/api/health", addr))
			if err != nil {
				lastErr = fmt.Errorf("unable to connect to api: %w", err)
			} else if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
	}
}

func markUIInstallComplete(password string, managerPort int) error {
	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: nil, // This is a local client so no proxy is needed
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	apiClient := apiclient.New(
		fmt.Sprintf("https://localhost:%d", managerPort),
		apiclient.WithHTTPClient(httpClient),
	)
	if err := apiClient.Authenticate(password); err != nil {
		return fmt.Errorf("unable to authenticate: %w", err)
	}

	_, err := apiClient.SetInstallStatus(apitypes.Status{
		State:       apitypes.InstallationStateSucceeded,
		Description: "Install Complete",
		LastUpdated: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("unable to set install status: %w", err)
	}

	return nil
}

func getManagerURL(hostname string, port int) string {
	if hostname != "" {
		return fmt.Sprintf("https://%s:%v", hostname, port)
	}
	ipaddr := runtimeconfig.TryDiscoverPublicIP()
	if ipaddr == "" {
		if addr := os.Getenv("EC_PUBLIC_ADDRESS"); addr != "" {
			ipaddr = addr
		} else {
			logrus.Errorf("Unable to determine node IP address")
			ipaddr = "NODE-IP-ADDRESS"
		}
	}
	return fmt.Sprintf("https://%s:%v", ipaddr, port)
}
