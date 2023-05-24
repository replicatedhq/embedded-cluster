/*
Package manager implements the component manager
*/
package manager

import (
	"container/list"
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// Manager manages components
type Manager struct {
	Components        []Component
	ReadyWaitDuration time.Duration

	started *list.List
}

// New creates a manager
func New() *Manager {
	return &Manager{
		Components:        []Component{},
		ReadyWaitDuration: 2 * time.Minute,
		started:           list.New(),
	}
}

// Add adds a component to the manager
func (m *Manager) Add(component Component) {
	m.Components = append(m.Components, component)
}

// Init initializes all managed components
func (m *Manager) Init(ctx context.Context) error {
	g, _ := errgroup.WithContext(ctx)

	for _, comp := range m.Components {
		compName := reflect.TypeOf(comp).Elem().Name()
		logrus.Infof("initializing %v", compName)
		c := comp
		// init this async
		g.Go(func() error {
			return c.Init(ctx)
		})
	}
	err := g.Wait()
	return err
}

// Start starts all managed components
func (m *Manager) Start(ctx context.Context) error {
	for _, comp := range m.Components {
		compName := reflect.TypeOf(comp).Elem().Name()
		logrus.Infof("starting %v", compName)
		if err := comp.Start(ctx); err != nil {
			_ = m.Stop()
			return err
		}
		m.started.PushFront(comp)
		if err := waitForReady(ctx, comp, compName, m.ReadyWaitDuration); err != nil {
			_ = m.Stop()
			return err
		}
	}
	return nil
}

// Stop stops all managed components
func (m *Manager) Stop() error {
	var ret error
	var next *list.Element

	for e := m.started.Front(); e != nil; e = next {
		component := e.Value.(Component)
		name := reflect.TypeOf(component).Elem().Name()

		if err := component.Stop(); err != nil {
			logrus.Errorf("failed to stop component %s: %s", name, err.Error())
			if ret == nil {
				ret = fmt.Errorf("failed to stop components")
			}
		} else {
			logrus.Infof("stopped component %s", name)
		}

		next = e.Next()
		m.started.Remove(e)
	}
	return ret
}

// waitForReady waits until the component is ready and returns true upon success. If a timeout occurs, it returns false
func waitForReady(ctx context.Context, comp Component, name string, timeout time.Duration) error {
	compWithReady, ok := comp.(Ready)
	if !ok {
		return nil
	}

	ctx, cancelFunction := context.WithTimeout(ctx, timeout)

	// clear up context after timeout
	defer cancelFunction()

	// loop forever, until the context is canceled or until etcd is healthy
	ticker := time.NewTicker(100 * time.Millisecond)
	l := logrus.WithField("component", name)
	for {
		l.Debugf("waiting for component readiness")
		if err := compWithReady.Ready(); err != nil {
			l.WithError(err).Debugf("component not ready yet")
		} else {
			return nil
		}

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			return fmt.Errorf("%s health-check timed out", name)
		}
	}
}
