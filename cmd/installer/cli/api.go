package cli

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	apiclient "github.com/replicatedhq/embedded-cluster/api/client"
	apilogger "github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/tlsutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/web"
	"github.com/sirupsen/logrus"
)

// apiConfig holds the configuration for the API server
type apiConfig struct {
	RuntimeConfig   runtimeconfig.RuntimeConfig
	Logger          logrus.FieldLogger
	MetricsReporter metrics.ReporterInterface
	Password        string
	ManagerPort     int
	LicenseFile     string
	AirgapBundle    string
	ConfigChan      chan<- *apitypes.InstallationConfig
	ReleaseData     *release.ReleaseData
	WebAssetsFS     fs.FS
}

func startAPI(ctx context.Context, cert tls.Certificate, config apiConfig) error {
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

func serveAPI(ctx context.Context, listener net.Listener, cert tls.Certificate, config apiConfig) error {
	router := mux.NewRouter()

	if config.ReleaseData == nil {
		return fmt.Errorf("release not found")
	}
	if config.ReleaseData.Application == nil {
		return fmt.Errorf("application not found")
	}

	logger, err := loggerFromConfig(config)
	if err != nil {
		return fmt.Errorf("new api logger: %w", err)
	}

	api, err := api.New(
		config.Password,
		api.WithLogger(logger),
		api.WithRuntimeConfig(config.RuntimeConfig),
		api.WithMetricsReporter(config.MetricsReporter),
		api.WithReleaseData(config.ReleaseData),
		api.WithLicenseFile(config.LicenseFile),
		api.WithAirgapBundle(config.AirgapBundle),
		api.WithConfigChan(config.ConfigChan),
	)
	if err != nil {
		return fmt.Errorf("new api: %w", err)
	}

	webServer, err := web.New(web.InitialState{
		Title: config.ReleaseData.Application.Spec.Title,
		Icon:  config.ReleaseData.Application.Spec.Icon,
	}, web.WithLogger(logger), web.WithAssetsFS(config.WebAssetsFS))
	if err != nil {
		return fmt.Errorf("new web server: %w", err)
	}

	api.RegisterRoutes(router.PathPrefix("/api").Subrouter())
	webServer.RegisterRoutes(router.PathPrefix("/").Subrouter())

	server := &http.Server{
		// ErrorLog outputs TLS errors and warnings to the console, we want to make sure we use the same logrus logger for them
		ErrorLog:  log.New(logger.WithField("http-server", "std-log").Writer(), "", 0),
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

func loggerFromConfig(config apiConfig) (logrus.FieldLogger, error) {
	if config.Logger != nil {
		return config.Logger, nil
	}
	logger, err := apilogger.NewLogger()
	if err != nil {
		return nil, fmt.Errorf("new api logger: %w", err)
	}
	return logger, nil
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
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			if lastErr != nil {
				return fmt.Errorf("api did not start in time: %w", lastErr)
			}
			return fmt.Errorf("api did not start in time")
		case <-time.Tick(1 * time.Second):
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://%s/api/health", addr), nil)
			if err != nil {
				lastErr = fmt.Errorf("unable to create request: %w", err)
				continue
			}
			resp, err := httpClient.Do(req)
			if err != nil {
				lastErr = fmt.Errorf("unable to connect to api: %w", err)
			} else if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
	}
}

func markUIInstallComplete(password string, managerPort int, installErr error) error {
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

	var state apitypes.State
	var description string
	if installErr != nil {
		state = apitypes.StateFailed
		description = fmt.Sprintf("Installation failed: %v", installErr)
	} else {
		state = apitypes.StateSucceeded
		description = "Installation succeeded"
	}

	_, err := apiClient.SetInstallStatus(&apitypes.Status{
		State:       state,
		Description: description,
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
