package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/metrics/types"
	"github.com/stretchr/testify/assert"
)

func TestSend(t *testing.T) {
	for _, tt := range []struct {
		name  string
		event types.Event
	}{
		{
			name: "InstallationStarted",
			event: types.InstallationStarted{
				GenericEvent: types.GenericEvent{
					ExecutionID:  "test-id",
					ClusterID:    "123",
					Version:      "1.2.3",
					EntryCommand: "install",
					Flags:        "--foo --bar --baz",
				},
				BinaryName: "bar",
				Type:       "baz",
				LicenseID:  "qux",
			},
		},
		{
			name: "InstallationSucceeded",
			event: types.InstallationSucceeded{
				GenericEvent: types.GenericEvent{
					ExecutionID:  "test-id",
					ClusterID:    "123",
					Version:      "1.2.3",
					EntryCommand: "install",
					Flags:        "--foo --bar --baz",
				},
			},
		},
		{
			name: "InstallationFailed",
			event: types.InstallationFailed{
				GenericEvent: types.GenericEvent{
					ExecutionID:  "test-id",
					ClusterID:    "123",
					Version:      "1.2.3",
					EntryCommand: "install",
					Flags:        "--foo --bar --baz",
					Reason:       "foo",
				},
			},
		},
		{
			name: "JoinStarted",
			event: types.JoinStarted{
				GenericEvent: types.GenericEvent{
					ExecutionID:  "test-id",
					ClusterID:    "123",
					Version:      "1.2.3",
					EntryCommand: "join",
					Flags:        "--foo --bar --baz",
				},
				NodeName: "foo",
			},
		},
		{
			name: "JoinSucceeded",
			event: types.JoinSucceeded{
				GenericEvent: types.GenericEvent{
					ExecutionID:  "test-id",
					ClusterID:    "123",
					Version:      "1.2.3",
					EntryCommand: "join",
					Flags:        "--foo --bar --baz",
				},
				NodeName: "foo",
			},
		},
		{
			name: "JoinFailed",
			event: types.JoinFailed{
				GenericEvent: types.GenericEvent{
					ExecutionID:  "test-id",
					ClusterID:    "123",
					Version:      "1.2.3",
					EntryCommand: "join",
					Flags:        "--foo --bar --baz",
					Reason:       "bar",
				},
				NodeName: "foo",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			payload := map[string]interface{}{"event": tt.event, "versions": map[string]string{"EmbeddedCluster": "v0.0.0", "Kubernetes": "0.0.0"}}
			expected, err := json.Marshal(payload)
			assert.NoError(t, err)
			server := httptest.NewServer(
				http.HandlerFunc(
					func(rw http.ResponseWriter, req *http.Request) {
						evname := reflect.TypeOf(tt.event).Name()
						path := fmt.Sprintf("/embedded_cluster_metrics/%s", evname)
						assert.Equal(t, req.URL.Path, path)
						assert.Equal(t, "POST", req.Method)
						received, err := io.ReadAll(req.Body)
						assert.NoError(t, err)
						assert.Equal(t, expected, received)
						var decoded = map[string]interface{}{}
						err = json.Unmarshal(received, &decoded)
						assert.NoError(t, err)
						assert.Contains(t, decoded, "event")
						rw.Write([]byte(`OK`))
					},
				),
			)
			defer server.Close()
			Send(context.Background(), server.URL, tt.event)
		})
	}
}
