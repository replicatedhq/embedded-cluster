package dryrun

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics/types"
	"github.com/sirupsen/logrus"
)

type Sender struct{}

var _ metrics.SenderInterface = (*Sender)(nil)

func (s *Sender) Send(ctx context.Context, baseURL string, ev types.Event) {
	url := metrics.EventURL(baseURL, ev)
	payload, err := metrics.EventPayload(ev)
	if err != nil {
		logrus.Debugf("unable to get payload for event %s: %s", ev.Title(), err)
		return
	}
	RecordMetric(ev.Title(), url, payload)
}
