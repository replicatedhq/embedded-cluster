package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
)

// Send is a helper function that sends an event to the metrics endpoint.
// Metrics endpoint can be overwritten by the license.spec.endpoint field
// or by the EMBEDDED_CLUSTER_METRICS_BASEURL environment variable, the latter has
// precedence over the former.
func Send(ctx context.Context, baseURL string, ev Event) {
	sender := Sender{baseURL}
	sender.Send(ctx, ev)
}

// Sender sends events to the metrics endpoint.
type Sender struct {
	baseURL string
}

// Send sends an event to the metrics endpoint.
func (s *Sender) Send(ctx context.Context, ev Event) {
	url := fmt.Sprintf("%s/embedded_cluster_metrics/%s", s.baseURL, ev.Title())
	payload, err := s.payload(ev)
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

// payload returns the payload to be sent to the metrics endpoint.
func (s *Sender) payload(ev Event) ([]byte, error) {
	payload := map[string]Event{"event": ev}
	return json.Marshal(payload)
}
