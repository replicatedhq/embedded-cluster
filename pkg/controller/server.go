package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/replicatedhq/helmbin/pkg/assets"
	"github.com/replicatedhq/helmbin/pkg/config"
	"github.com/replicatedhq/helmbin/pkg/server"
	"github.com/replicatedhq/helmbin/static"
)

// Server implement the component interface to run the helmbin server
type Server struct {
	Options config.CLIOptions

	ctx    context.Context
	cancel context.CancelFunc
}

// Init initializes the server
func (k *Server) Init(_ context.Context) error {
	err := os.RemoveAll(filepath.Join(k.Options.DataDir, "server/static"))
	if err != nil {
		return fmt.Errorf("remove server/static: %w", err)
	}

	err = assets.Stage(static.FS(), k.Options.DataDir, "server/static", 0440)
	if err != nil {
		return fmt.Errorf("stage server/static: %w", err)
	}

	return nil
}

// Start starts the server
func (k *Server) Start(ctx context.Context) error {
	k.ctx, k.cancel = context.WithCancel(ctx)

	options := server.Options{
		Address:   ":10680",
		StaticDir: filepath.Join(k.Options.DataDir, "server/static"),
	}
	return server.StartServer(k.ctx, options)
}

// Stop stops the server
func (k *Server) Stop() error {
	k.cancel()
	return nil
}

// Ready is the health-check interface
func (k *Server) Ready() error {
	// TODO
	return nil
}
