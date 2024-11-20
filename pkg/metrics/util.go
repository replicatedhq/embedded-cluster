package metrics

import (
	"encoding/json"
	"fmt"
	"strings"

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

var secretKeywords = []string{"password", "secret", "token", "key"}

// redactFlags redacts presumed secret values from a string slice based on preset keywords.
// For example, given ["--license", "./path", "--password", "123"], it will return ["--license", "./path", "--password", "*****"]
func redactFlags(flags []string) []string {
	result := make([]string, len(flags))

	valueHasSecret := false
	for i, flag := range flags {
		// Check if the is the flag name or the value
		isFlagKey := strings.HasPrefix(flag, "-")
		// This is a flag value and the previous iteration detected one of the secret keywords, let's redact it
		if !isFlagKey && valueHasSecret {
			result[i] = "*****"
			continue
		}
		result[i] = flag
		// This is a flag value no point in checking it for the secret keywords
		if !isFlagKey {
			continue
		}
		for _, keyword := range secretKeywords {
			if valueHasSecret = strings.Contains(flag, keyword); valueHasSecret {
				break
			}
		}
	}
	return result
}
