package metrics

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/metrics/types"
	"github.com/stretchr/testify/assert"
)

func TestSend(t *testing.T) {
	for _, tt := range []struct {
		name          string
		event         types.Event
		wantURLPath   string
		wantEventType string
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
					EventType:    "InstallationStarted",
				},
				BinaryName: "bar",
				LegacyType: "baz",
				LicenseID:  "qux",
			},
			wantURLPath:   "/embedded_cluster_metrics/InstallationStarted",
			wantEventType: "InstallationStarted",
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
					EventType:    "InstallationSucceeded",
				},
			},
			wantURLPath:   "/embedded_cluster_metrics/GenericEvent",
			wantEventType: "InstallationSucceeded",
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
					EventType:    "InstallationFailed",
				},
			},
			wantURLPath:   "/embedded_cluster_metrics/GenericEvent",
			wantEventType: "InstallationFailed",
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
					EventType:    "JoinStarted",
				},
				NodeName: "foo",
			},
			wantURLPath:   "/embedded_cluster_metrics/JoinStarted",
			wantEventType: "JoinStarted",
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
					EventType:    "JoinSucceeded",
				},
				NodeName: "foo",
			},
			wantURLPath:   "/embedded_cluster_metrics/JoinSucceeded",
			wantEventType: "JoinSucceeded",
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
					EventType:    "JoinFailed",
				},
				NodeName: "foo",
			},
			wantURLPath:   "/embedded_cluster_metrics/JoinFailed",
			wantEventType: "JoinFailed",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			payload := map[string]interface{}{"event": tt.event, "versions": map[string]string{"EmbeddedCluster": "v0.0.0", "Kubernetes": "0.0.0"}}
			expected, err := json.Marshal(payload)
			assert.NoError(t, err)
			server := httptest.NewServer(
				http.HandlerFunc(
					func(rw http.ResponseWriter, req *http.Request) {
						assert.Equal(t, tt.wantURLPath, req.URL.Path)
						assert.Equal(t, "POST", req.Method)
						received, err := io.ReadAll(req.Body)
						assert.NoError(t, err)
						assert.Equal(t, expected, received)
						var decoded = map[string]interface{}{}
						err = json.Unmarshal(received, &decoded)
						assert.NoError(t, err)
						assert.Contains(t, decoded, "event")
						assert.Equal(t, tt.wantEventType, decoded["event"].(map[string]interface{})["eventType"])
						rw.Write([]byte(`OK`))
					},
				),
			)
			defer server.Close()
			Send(context.Background(), server.URL, tt.event)
		})
	}
}
