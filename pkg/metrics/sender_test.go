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

	"github.com/google/uuid"
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
				ClusterID:  uuid.New(),
				Version:    "1.2.3",
				Flags:      "foo",
				BinaryName: "bar",
				Type:       "baz",
				LicenseID:  "qux",
			},
		},
		{
			name: "InstallationSucceeded",
			event: types.InstallationSucceeded{
				ClusterID: uuid.New(),
			},
		},
		{
			name: "InstallationFailed",
			event: types.InstallationFailed{
				ClusterID: uuid.New(),
				Reason:    "foo",
			},
		},
		{
			name: "JoinStarted",
			event: types.JoinStarted{
				ClusterID: uuid.New(),
				NodeName:  "foo",
			},
		},
		{
			name: "JoinSucceeded",
			event: types.JoinSucceeded{
				ClusterID: uuid.New(),
				NodeName:  "foo",
			},
		},
		{
			name: "JoinFailed",
			event: types.JoinFailed{
				ClusterID: uuid.New(),
				NodeName:  "foo",
				Reason:    "bar",
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
