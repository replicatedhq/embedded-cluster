package metrics

import (
	"bytes"
	"context"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/pkg/metrics/types"
	"github.com/sirupsen/logrus"
)

// Send sends an event to the metrics endpoint.
func (s *Sender) Send(ctx context.Context, baseURL string, ev types.Event) {
	url := EventURL(baseURL, ev)
	payload, err := EventPayload(ev)
	if err != nil {
		logrus.Debugf("unable to get payload for event %s: %s", ev.Title(), err)
		return
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		logrus.Debugf("unable to create request for event %s: %s", ev.Title(), err)
		return
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request.WithContext(ctx))
	if err != nil {
		logrus.Debugf("unable to send event %s: %s", ev.Title(), err)
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		logrus.Debugf("unable to confirm event %s: %d", ev.Title(), response.StatusCode)
	}
}
