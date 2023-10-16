package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/sirupsen/logrus"

	"github.com/replicatedhq/helmvm/pkg/customization"
)

// Send is a helper function that sends an event to the metrics endpoint.
// Metrics endpoint can be overwritten by the license.spec.endpoint field
// or by the HELMVM_METRICS_BASEURL environment variable, the latter has
// precedence over the former.
func Send(ctx context.Context, ev Event) {
	var baseURL = "https://replicated.app"
	var custom customization.AdminConsoleCustomization
	if license, err := custom.License(); err != nil {
		logrus.Warnf("unable to read license: %s", err)
		return
	} else if license != nil && license.Spec.Endpoint != "" {
		baseURL = license.Spec.Endpoint
	}
	if os.Getenv("HELMVM_METRICS_BASEURL") != "" {
		baseURL = os.Getenv("HELMVM_METRICS_BASEURL")
	}
	sender := Sender{baseURL}
	sender.Send(ctx, ev)
}

// Sender sends events to the metrics endpoint.
type Sender struct {
	baseURL string
}

// Send sends an event to the metrics endpoint.
func (s *Sender) Send(ctx context.Context, ev Event) {
	url := fmt.Sprintf("%s/helmbin_metrics/%s", s.baseURL, ev.Title())
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
