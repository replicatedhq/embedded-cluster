/*
Package server implements a static HTTP file server for the application.
*/
package server

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/sirupsen/logrus"
)

// Options is the configuration for the server
type Options struct {
	// Address is the address to listen on
	Address string

	// StaticDir is the path to the static files directory
	StaticDir string
}

// StartServer starts the server
func StartServer(ctx context.Context, opts Options) error {
	listener, err := net.Listen("tcp", opts.Address)
	if err != nil {
		return err
	}

	router := http.NewServeMux()
	router.Handle("/static/", serveStatic("/static/", opts.StaticDir))

	server := http.Server{
		Handler: router,
	}

	logrus.Infof("Starting server on %s", opts.Address)

	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logrus.Fatalf("Server stopped: %v", err)
		}
	}()

	// Shutdown the http server when the context is closed
	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()
	return nil
}
