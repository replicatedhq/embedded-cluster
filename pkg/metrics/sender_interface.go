package metrics

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/pkg/metrics/types"
	"github.com/sirupsen/logrus"
)

var s SenderInterface

// Sender sends events to the metrics endpoint.
type Sender struct{}

var _ SenderInterface = (*Sender)(nil)

func init() {
	Set(&Sender{})
}

func Set(_s SenderInterface) {
	s = _s
}

type SenderInterface interface {
	Send(ctx context.Context, baseURL string, ev types.Event)
}

// Convenience functions

// Send is a helper function that sends an event to the metrics endpoint.
// Metrics endpoint can be overwritten by the license.spec.endpoint field.
func Send(ctx context.Context, baseURL string, ev types.Event) {
	if metricsDisabled {
		logrus.Debugf("Metrics are disabled, not sending event %s", ev.Title())
		return
	} else if baseURL == "" {
		logrus.Debugf("Metrics base URL empty, not sending event %s", ev.Title())
		return
	}
	s.Send(ctx, baseURL, ev)
}
