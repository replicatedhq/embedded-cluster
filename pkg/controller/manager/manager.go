/*
Package manager implements the component manager
*/
package manager

import (
	"container/list"
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// Manager manages components
type Manager struct {
	Components        []Component
	ReadyWaitDuration time.Duration

	started *list.List
}

// New creates a manager.
func New() *Manager {
	return &Manager{
		Components:        []Component{},
		ReadyWaitDuration: 2 * time.Minute,
		started:           list.New(),
	}
}

// Add adds a component to the manager.
func (m *Manager) Add(component Component) {
	m.Components = append(m.Components, component)
}

// Init initializes all managed components.
func (m *Manager) Init(ctx context.Context) error {
	g, _ := errgroup.WithContext(ctx)
	for _, comp := range m.Components {
		logrus.Infof("Initializing component %T", comp)
		c := comp
		g.Go(func() error { return c.Init(ctx) })
	}
	return g.Wait()
}

// Start starts all managed components.
func (m *Manager) Start(ctx context.Context) error {
	for _, comp := range m.Components {
		logrus.Infof("Starting component %T", comp)
		if err := comp.Start(ctx); err != nil {
			_ = m.Stop()
			return err
		}
		m.started.PushFront(comp)
		if err := waitForReady(ctx, comp, m.ReadyWaitDuration); err != nil {
			_ = m.Stop()
			return err
		}
	}
	return nil
}

// Stop stops all managed components.
func (m *Manager) Stop() error {
	var result *multierror.Error
	var next *list.Element
	for e := m.started.Front(); e != nil; e = next {
		comp := e.Value.(Component)
		if err := comp.Stop(); err != nil {
			logrus.Errorf("Failed to stop component %T: %s", comp, err.Error())
			result = multierror.Append(result, err)
			next = e.Next()
			m.started.Remove(e)
			continue
		}
		logrus.Infof("Stopped component %T", comp)
		next = e.Next()
		m.started.Remove(e)
	}
	return result.ErrorOrNil()
}

// waitForReady waits until the component is ready. if a timeout occurs an error is
// returned instead.
func waitForReady(ctx context.Context, comp Component, timeout time.Duration) error {
	assessable, ok := comp.(Ready)
	if !ok {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// loop forever, until the context is canceled or until etcd is healthy
	ticker := time.NewTicker(100 * time.Millisecond)
	log := logrus.WithField("component", fmt.Sprintf("%T", comp))
	for {
		log.Debugf("Waiting for component readiness")
		if err := assessable.Ready(); err == nil {
			return nil
		}
		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			return fmt.Errorf("%T component health-check timed out", comp)
		}
	}
}
