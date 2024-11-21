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
// For example, given ["--license", "./path", "--password", "123", "--a-secret=123"]
// it will return ["--license", "./path", "--password", "*****", "--a-secret=*****"]"
func redactFlags(flags []string) []string {
	result := make([]string, len(flags))

	valueHasSecret := false
	for i, flag := range flags {
		// The previous iteration detected one of the secret keywords on flag key, let's redact this value
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
			flagParts := strings.Split(flag, "=")
			key := flagParts[0]
			value := flagParts[1]

			// If the key has a secret keyword and the value is not empty, redact it
			if hasSecretKeyword(key) && len(value) > 0 {
				result[i] = key + "=*****"
			} else {
				result[i] = flag
			}
			continue
		}

		result[i] = flag
		// This is a flag value no point in checking it for the secret keywords
		if !isFlagKey {
			continue
		}
		valueHasSecret = hasSecretKeyword(flag)
	}
	return result
}

func hasSecretKeyword(value string) bool {
	for _, keyword := range secretKeywords {
		if strings.Contains(value, keyword) {
			return true
		}
	}
	return false
}
