package cli

import (
	"context"
	"crypto/tls"
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
	"github.com/replicatedhq/embedded-cluster/pkg-new/cloudutils"
	"github.com/replicatedhq/embedded-cluster/pkg-new/tlsutils"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/web"
	"github.com/sirupsen/logrus"
)

// apiOptions holds the configuration options for the API server
type apiOptions struct {
	apitypes.APIConfig

	ManagerPort int
	Headless    bool
	// The mode the web will be running on, install or upgrade
	WebMode web.Mode

	Logger          logrus.FieldLogger
	MetricsReporter metrics.ReporterInterface
	WebAssetsFS     fs.FS
}

// startAPI starts the API server and returns a channel that will receive an error if the API exits unexpectedly.
// The returned channel will be closed when the API exits normally (via context cancellation).
func startAPI(ctx context.Context, cert tls.Certificate, opts apiOptions) (<-chan error, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", opts.ManagerPort))
	if err != nil {
		return nil, fmt.Errorf("unable to create listener: %w", err)
	}
	logrus.Debugf("API server listening on port: %d", opts.ManagerPort)

	addr := fmt.Sprintf("https://localhost:%d", opts.ManagerPort)
	httpClient := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			Proxy: nil,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	apiExitCh := make(chan error, 1)
	waitErrCh := make(chan error, 1)

	go func() {
		apiExitCh <- serveAPI(ctx, listener, cert, opts)
	}()

	go func() {
		waitErrCh <- waitForAPI(ctx, httpClient, addr)
	}()

	select {
	case err := <-apiExitCh:
		if err != nil {
			return nil, fmt.Errorf("serve api: %w", err)
		}
	case err := <-waitErrCh:
		if err != nil {
			return apiExitCh, fmt.Errorf("wait for api: %w", err)
		}
	}

	return apiExitCh, nil
}

func serveAPI(ctx context.Context, listener net.Listener, cert tls.Certificate, opts apiOptions) error {
	router := mux.NewRouter()

	if opts.ReleaseData == nil {
		return fmt.Errorf("release not found")
	}
	if opts.ReleaseData.Application == nil {
		return fmt.Errorf("application not found")
	}

	if dryrun.Enabled() {
		logger := logrus.New() // log to stdout for dryrun
		opts.Logger = logger
	}

	logger, err := loggerFromOptions(opts)
	if err != nil {
		return fmt.Errorf("new api logger: %w", err)
	}

	apiOpts := []api.Option{
		api.WithLogger(logger),
		api.WithMetricsReporter(opts.MetricsReporter),
	}

	if dryrun.Enabled() {
		hcli, err := helm.NewClient(helm.HelmOptions{})
		if err != nil {
			return fmt.Errorf("create dryrun helm client: %w", err)
		}
		apiOpts = append(apiOpts, api.WithHelmClient(hcli))

		kcli, err := kubeutils.KubeClient()
		if err != nil {
			return fmt.Errorf("create dryrun kube client: %w", err)
		}
		apiOpts = append(apiOpts, api.WithKubeClient(kcli))

		metadataClient, err := kubeutils.MetadataClient()
		if err != nil {
			return fmt.Errorf("create dryrun metadata client: %w", err)
		}
		apiOpts = append(apiOpts, api.WithMetadataClient(metadataClient))
	}

	api, err := api.New(opts.APIConfig, apiOpts...)
	if err != nil {
		return fmt.Errorf("new api: %w", err)
	}

	api.RegisterRoutes(router.PathPrefix("/api").Subrouter())

	// Only start web server for UI mode, not headless
	if !opts.Headless {
		webServer, err := web.New(web.InitialState{
			Title:                opts.ReleaseData.Application.Spec.Title,
			Icon:                 opts.ReleaseData.Application.Spec.Icon,
			InstallTarget:        string(opts.InstallTarget),
			Mode:                 opts.WebMode,
			IsAirgap:             opts.AirgapBundle != "",
			RequiresInfraUpgrade: opts.RequiresInfraUpgrade,
		}, web.WithLogger(logger), web.WithAssetsFS(opts.WebAssetsFS))
		if err != nil {
			return fmt.Errorf("new web server: %w", err)
		}
		webServer.RegisterRoutes(router.PathPrefix("/").Subrouter())
	}

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

func waitForAPI(ctx context.Context, httpClient *http.Client, addr string) error {
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
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/health", addr), nil)
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
