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
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/spf13/cobra"
)

var (
	whitelistServeDirs = []string{"bin", "charts", "images"}
)

// serveCommand starts a http server that serves files from the data directory. This server listen
// only on localhost and is used to serve files needed by the autopilot during an upgrade.
func ServeCmd(cli *CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve files from the data directory over HTTP",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			cli.bindFlags(cmd.Flags())

			// If the command is help, don't require root
			if cmd.Name() == "help" {
				return nil
			}

			if cli.ServeRequiresRoot && os.Getuid() != 0 {
				return fmt.Errorf("serve command must be run as root")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			port := cli.V.GetInt("port")

			handler := http.NewServeMux()

			fileServer := http.FileServer(http.Dir(cli.RC.EmbeddedClusterHomeDirectory()))
			loggedFileServer := logAndFilterRequest(fileServer)
			handler.Handle("/", loggedFileServer)

			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			if err := startBinaryWatcher(cancel, cli.RC); err != nil {
				panic(err)
			}

			addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
			server := &http.Server{Addr: addr, Handler: handler}
			go func() {
				fmt.Printf("Starting server on %s\n", addr)
				if err := server.ListenAndServe(); err != nil {
					if err != http.ErrServerClosed {
						panic(err)
					}
				}
			}()

			<-ctx.Done()
			fmt.Println("Shutting down server...")

			// Shutdown the server with a 10 second grace period
			ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := server.Shutdown(ctx); err != nil {
				panic(err)
			}
			fmt.Println("Server gracefully stopped")
			return nil
		},
	}

	cmd.Flags().Int("port", ecv1beta1.DefaultLocalArtifactMirrorPort, "Port to listen on")

	return cmd
}

// startBinaryWatcher starts a loop that observes the binary until its modification
// time changes. When the modification time changes a SIGTERM is send in the provided
// channel.
func startBinaryWatcher(cancel context.CancelFunc, rc runtimeconfig.RuntimeConfig) error {
	fpath := rc.PathToEmbeddedClusterBinary("local-artifact-mirror")
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
			cancel()
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
