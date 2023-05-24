package controller

import (
	"context"

	"github.com/emosbaugh/helmbin/pkg/config"
)

// Helm implement the component interface to run the Helm controller
type Helm struct {
	Config config.Config

	ctx    context.Context
	cancel context.CancelFunc
}

// Init initializes the Helm controller
func (k *Helm) Init(_ context.Context) error {
	return nil
}

// Start starts the Helm controller
func (k *Helm) Start(ctx context.Context) error {
	k.ctx, k.cancel = context.WithCancel(ctx)
	return nil
}

// Stop stops the Helm controller
func (k *Helm) Stop() error {
	k.cancel()
	return nil
}

// Ready is the health-check interface
func (k *Helm) Ready() error {
	// TODO
	return nil
}
