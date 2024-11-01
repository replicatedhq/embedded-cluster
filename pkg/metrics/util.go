package metrics

import (
	"encoding/json"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/pkg/metrics/types"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
)

// EventURL returns the URL to be used when sending an event to the metrics endpoint.
func EventURL(baseURL string, ev types.Event) string {
	return fmt.Sprintf("%s/embedded_cluster_metrics/%s", baseURL, ev.Title())
}

// EventPayload returns the payload to be sent to the metrics endpoint.
func EventPayload(ev types.Event) ([]byte, error) {
	vmap := map[string]string{
		"EmbeddedCluster": versions.Version,
		"Kubernetes":      versions.K0sVersion,
	}
	payload := map[string]interface{}{"event": ev, "versions": vmap}
	return json.Marshal(payload)
}
