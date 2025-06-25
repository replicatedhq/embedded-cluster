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
	apilogger "github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/cloudutils"
	"github.com/replicatedhq/embedded-cluster/pkg-new/tlsutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/web"
	"github.com/sirupsen/logrus"
)

// apiOptions holds the configuration options for the API server
type apiOptions struct {
	InstallTarget             string
	RuntimeConfig             runtimeconfig.RuntimeConfig
	Logger                    logrus.FieldLogger
	MetricsReporter           metrics.ReporterInterface
	Password                  string
	TLSConfig                 apitypes.TLSConfig
	ManagerPort               int
	License                   []byte
	AirgapBundle              string
	ConfigValues              string
	ReleaseData               *release.ReleaseData
	EndUserConfig             *ecv1beta1.Config
	AllowIgnoreHostPreflights bool
	WebAssetsFS               fs.FS
}

func startAPI(ctx context.Context, cert tls.Certificate, opts apiOptions) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", opts.ManagerPort))
	if err != nil {
		return fmt.Errorf("unable to create listener: %w", err)
	}

	go func() {
		if err := serveAPI(ctx, listener, cert, opts); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				logrus.Errorf("api error: %v", err)
			}
		}
	}()

	addr := fmt.Sprintf("localhost:%d", opts.ManagerPort)
	if err := waitForAPI(ctx, addr); err != nil {
		return fmt.Errorf("unable to wait for api: %w", err)
	}

	return nil
}

func serveAPI(ctx context.Context, listener net.Listener, cert tls.Certificate, opts apiOptions) error {
	router := mux.NewRouter()

	if opts.ReleaseData == nil {
		return fmt.Errorf("release not found")
	}
	if opts.ReleaseData.Application == nil {
		return fmt.Errorf("application not found")
	}

	logger, err := loggerFromOptions(opts)
	if err != nil {
		return fmt.Errorf("new api logger: %w", err)
	}

	cfg := apitypes.APIConfig{
		RuntimeConfig:             opts.RuntimeConfig,
		Password:                  opts.Password,
		TLSConfig:                 opts.TLSConfig,
		License:                   opts.License,
		AirgapBundle:              opts.AirgapBundle,
		ConfigValues:              opts.ConfigValues,
		ReleaseData:               opts.ReleaseData,
		EndUserConfig:             opts.EndUserConfig,
		AllowIgnoreHostPreflights: opts.AllowIgnoreHostPreflights,
	}

	api, err := api.New(
		cfg,
		api.WithLogger(logger),
		api.WithMetricsReporter(opts.MetricsReporter),
	)
	if err != nil {
		return fmt.Errorf("new api: %w", err)
	}

	webServer, err := web.New(web.InitialState{
		Title:         opts.ReleaseData.Application.Spec.Title,
		Icon:          opts.ReleaseData.Application.Spec.Icon,
		InstallTarget: opts.InstallTarget,
	}, web.WithLogger(logger), web.WithAssetsFS(opts.WebAssetsFS))
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
		_ = server.Shutdown(context.Background())
	}()

	return server.ServeTLS(listener, "", "")
}

func loggerFromOptions(opts apiOptions) (logrus.FieldLogger, error) {
	if opts.Logger != nil {
		return opts.Logger, nil
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

func getManagerURL(hostname string, port int) string {
	if hostname != "" {
		return fmt.Sprintf("https://%s:%v", hostname, port)
	}
	ipaddr := cloudutils.TryDiscoverPublicIP()
	if ipaddr == "" {
		if addr := os.Getenv("EC_PUBLIC_ADDRESS"); addr != "" {
			ipaddr = addr
		} else {
			logrus.Warnf("\nUnable to automatically determine the node's IP address. Replace <node-ip> in the URL below with your node's IP address.")
			ipaddr = "<node-ip>"
		}
	}
	return fmt.Sprintf("https://%s:%v", ipaddr, port)
}
