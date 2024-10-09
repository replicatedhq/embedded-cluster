package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/urfave/cli/v2"
	k8snet "k8s.io/utils/net"
)

var (
	whitelistServeDirs = []string{"bin", "charts", "images"}
)

// serveCommand starts a http server that serves files from the data directory. This server listen
// only on localhost and is used to serve files needed by the autopilot during an upgrade.
var serveCommand = &cli.Command{
	Name:  "serve",
	Usage: "Serve files from the data directory over HTTP",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "data-dir",
			Usage:   "Path to the data directory",
			Value:   ecv1beta1.DefaultDataDir,
			EnvVars: []string{"LOCAL_ARTIFACT_MIRROR_DATA_DIR"},
		},
		&cli.StringFlag{
			Name:    "port",
			Usage:   "Port to listen on",
			Value:   strconv.Itoa(ecv1beta1.DefaultLocalArtifactMirrorPort),
			EnvVars: []string{"LOCAL_ARTIFACT_MIRROR_PORT"},
		},
	},
	Before: func(c *cli.Context) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("serve command must be run as root")
		}
		return nil
	},
	Action: func(c *cli.Context) error {
		var provider *defaults.Provider
		if c.IsSet("data-dir") {
			provider = defaults.NewProvider(c.String("data-dir"))
		} else {
			var err error
			provider, err = defaults.NewProviderFromFilesystem()
			if err != nil {
				panic(fmt.Errorf("unable to get provider from filesystem: %w", err))
			}
		}

		port, err := k8snet.ParsePort(c.String("port"), false)
		if err != nil {
			panic(fmt.Errorf("unable to parse port from flag: %w", err))
		}

		os.Setenv("TMPDIR", provider.EmbeddedClusterTmpSubDir())

		fileServer := http.FileServer(http.Dir(provider.EmbeddedClusterHomeDirectory()))
		loggedFileServer := logAndFilterRequest(fileServer)
		http.Handle("/", loggedFileServer)

		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

		if err := startBinaryWatcher(provider, stop); err != nil {
			panic(err)
		}

		addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		server := &http.Server{Addr: addr}
		go func() {
			fmt.Printf("Starting server on %s\n", addr)
			if err := server.ListenAndServe(); err != nil {
				if err != http.ErrServerClosed {
					panic(err)
				}
			}
		}()

		<-stop
		fmt.Println("Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			panic(err)
		}
		fmt.Println("Server gracefully stopped")
		return nil
	},
}

// startBinaryWatcher starts a loop that observes the binary until its modification
// time changes. When the modification time changes a SIGTERM is send in the provided
// channel.
func startBinaryWatcher(provider *defaults.Provider, stop chan os.Signal) error {
	fpath := provider.PathToEmbeddedClusterBinary("local-artifact-mirror")
	stat, err := os.Stat(fpath)
	if err != nil {
		return fmt.Errorf("unable to stat %s: %s", fpath, err)
	}
	lastmod := stat.ModTime()
	go func() {
		fmt.Println("Watching for changes in the binary")
		ticker := time.NewTicker(5 * time.Second)
		for range ticker.C {
			if stat, err = os.Stat(fpath); err != nil {
				fmt.Println("Unable to stat binary:", err)
				continue
			}
			if stat.ModTime().Equal(lastmod) {
				continue
			}
			fmt.Println("Binary changed, sending signal to stop")
			stop <- syscall.SIGTERM
		}
	}()
	return nil
}

// logAndFilterRequest is a middleware that logs the HTTP request details. Returns 404
// if attempting to read the log files as those are not served by this server.
func logAndFilterRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		for _, dir := range whitelistServeDirs {
			if !strings.HasPrefix(dir, "/") {
				dir = "/" + dir
			}
			if !strings.HasSuffix(dir, "/") {
				dir = dir + "/"
			}
			if strings.HasPrefix(r.URL.Path, dir) {
				fmt.Printf("serving %s\n", r.URL.Path)
				handler.ServeHTTP(w, r)
				return
			}
		}
		fmt.Printf("not serving %s\n", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	})
}
