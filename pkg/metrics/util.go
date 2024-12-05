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

var flagsToRedact = []string{"http-proxy", "https-proxy", "admin-console-password"}

// redactFlags redacts secret values from a string slice based on preset flags to redact.
// For example, given ["--license", "./path", "--admin-console-password", "123", --http-proxy=http://user:password@localhost:8080],
// it will return ["--license", "./path", "--admin-console-password", "*****", --http-proxy=*****].
func redactFlags(flags []string) []string {
	result := make([]string, len(flags))

	valueHasSecret := false
	for i, flag := range flags {
		// The previous iteration detected one of the flags to redact, let's redact this value
		if valueHasSecret {
			result[i] = "*****"
			valueHasSecret = false
			continue
		}
		// Check if this is the flag name or the value
		isFlagKey := strings.HasPrefix(flag, "-")
		// Flag is of the form --key=value
		if isFlagKey && strings.Contains(flag, "=") {

			// Split the flag into key and value
			flagParts := strings.SplitN(flag, "=", 2)
			key := flagParts[0]
			value := flagParts[1]

			// If the key is a flag to redact and the value is not empty, redact it
			if hasFlagToRedact(key) && len(value) > 0 {
				result[i] = key + "=*****"
			} else {
				result[i] = flag
			}
			continue
		}

		result[i] = flag
		// This is a flag value no point in checking it for secrets to redact
		if !isFlagKey {
			continue
		}
		valueHasSecret = hasFlagToRedact(flag)
	}
	return result
}

func hasFlagToRedact(value string) bool {
	for _, flag := range flagsToRedact {
		if strings.Contains(value, flag) {
			return true
		}
	}
	return false
}
