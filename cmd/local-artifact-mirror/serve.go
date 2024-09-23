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

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/urfave/cli/v2"
	k8snet "k8s.io/utils/net"
)

// serveCommand starts a http server that serves files from the /var/lib/embedded-cluster
// directory. This server listen only on localhost and is used to serve files needed by
// the autopilot during an upgrade.
var serveCommand = &cli.Command{
	Name:  "serve",
	Usage: "Serve /var/lib/embedded-cluster files over HTTP",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "port",
			Usage:   "Port to listen on",
			Value:   strconv.Itoa(defaults.LocalArtifactMirrorPort),
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
		dir := defaults.EmbeddedClusterHomeDirectory()

		fileServer := http.FileServer(http.Dir(dir))
		loggedFileServer := logAndFilterRequest(fileServer)
		http.Handle("/", loggedFileServer)

		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

		if err := startBinaryWatcher(stop); err != nil {
			panic(err)
		}

		port := defaults.LocalArtifactMirrorPort
		portStr := c.String("port")
		if portStr != "" {
			var err error
			port, err = k8snet.ParsePort(portStr, false)
			if err != nil {
				panic(fmt.Errorf("unable to parse port: %w", err))
			}
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
func startBinaryWatcher(stop chan os.Signal) error {
	fpath := defaults.PathToEmbeddedClusterBinary("local-artifact-mirror")
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
		if strings.HasPrefix(r.URL.Path, "/logs") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		handler.ServeHTTP(w, r)
	})
}
