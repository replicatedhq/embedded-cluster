package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
)

// serveCommand starts a http server that serves files from the /var/lib/embedded-cluster
// directory. This server listen only on localhost and is used to serve files needed by
// the autopilot during an upgrade.
var serveCommand = &cli.Command{
	Name:  "serve",
	Usage: "Serve /var/lib/embedded-cluster files over HTTP",
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

		server := &http.Server{Addr: "127.0.0.1:50000"}
		go func() {
			fmt.Println("Starting server on 127.0.0.1:50000")
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
