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
	cmdutil "github.com/replicatedhq/embedded-cluster/pkg/cmd/util"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	whitelistServeDirs = []string{"bin", "charts", "images"}
)

// serveCommand starts a http server that serves files from the data directory. This server listen
// only on localhost and is used to serve files needed by the autopilot during an upgrade.
func ServeCmd(ctx context.Context, v *viper.Viper) *cobra.Command {
	var (
		dataDir string
		port    int
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve files from the data directory over HTTP",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			v.BindPFlag("data-dir", cmd.Flags().Lookup("data-dir"))
			v.BindPFlag("port", cmd.Flags().Lookup("port"))

			if os.Getuid() != 0 {
				return fmt.Errorf("serve command must be run as root")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			// there are some env vars that we still support for backwards compatibility
			if os.Getenv("LOCAL_ARTIFACT_MIRROR_PORT") != "" {
				envvarPort, err := strconv.ParseInt(os.Getenv("LOCAL_ARTIFACT_MIRROR_PORT"), 10, 32)
				if err != nil {
					return fmt.Errorf("unable to parse LOCAL_ARTIFACT_MIRROR_PORT: %w", err)
				}
				port = int(envvarPort)
			}
			if os.Getenv("LOCAL_ARTIFACT_MIRROR_DATA_DIR") != "" {
				dataDir = os.Getenv("LOCAL_ARTIFACT_MIRROR_DATA_DIR")
			}

			var provider *defaults.Provider
			if v.Get("data-dir") != nil {
				provider = defaults.NewProvider(v.GetString("data-dir"))
			} else {
				var err error
				provider, err = cmdutil.NewProviderFromFilesystem()
				if err != nil {
					panic(fmt.Errorf("unable to get provider from filesystem: %w", err))
				}
			}

			port := v.GetInt("port")
			if port == 0 {
				port = ecv1beta1.DefaultLocalArtifactMirrorPort
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

	cmd.Flags().StringVar(&dataDir, "data-dir", ecv1beta1.DefaultDataDir, "Path to the data directory")
	cmd.Flags().IntVar(&port, "port", ecv1beta1.DefaultLocalArtifactMirrorPort, "Port to listen on")

	return cmd
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
