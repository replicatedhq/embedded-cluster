package metrics

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/metrics/types"
	"github.com/stretchr/testify/require"
)

func TestReportInstallationStarted(t *testing.T) {
	for _, test := range []struct {
		name            string
		OSArgs          []string
		validateRequest func(*testing.T, *http.Request)
	}{
		{
			name:   "redact secret flags",
			OSArgs: []string{"-l", "./license.yaml", "--admin-console-password", "some-password", "--skip-host-preflights", "--http-proxy", "http://user:password@my-proxy.test", "--https-proxy=https://user:password@my-https-proxy.test", "--admin-console-port", "8080"},
			validateRequest: func(t *testing.T, r *http.Request) {
				req := require.New(t)
				body, err := io.ReadAll(r.Body)
				req.NoError(err)

				var decoded map[string]json.RawMessage
				err = json.Unmarshal(body, &decoded)
				req.NoError(err)

				var event types.InstallationStarted
				err = json.Unmarshal(decoded["event"], &event)
				req.NoError(err)
				req.Equal("-l ./license.yaml --admin-console-password ***** --skip-host-preflights --http-proxy ***** --https-proxy=***** --admin-console-port 8080", event.Flags)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(
					func(rw http.ResponseWriter, req *http.Request) {
						test.validateRequest(t, req)
						rw.Write([]byte(`OK`))
					},
				),
			)
			defer server.Close()

			reporter := NewReporter("test-execution-id", server.URL, "123", "install", test.OSArgs)
			reporter.ReportInstallationStarted(context.Background(), "license-id", "app-slug")
		})
	}
}

func TestReportInstallationSucceeded(t *testing.T) {
	for _, test := range []struct {
		name            string
		validateRequest func(*testing.T, *http.Request)
	}{
		{
			name: "generic event",
			validateRequest: func(t *testing.T, r *http.Request) {
				req := require.New(t)
				body, err := io.ReadAll(r.Body)
				req.NoError(err)

				var decoded map[string]json.RawMessage
				err = json.Unmarshal(body, &decoded)
				req.NoError(err)

				var event types.GenericEvent
				err = json.Unmarshal(decoded["event"], &event)
				req.NoError(err)

				req.Equal("test-execution-id", event.ExecutionID)
				req.Equal("123", event.ClusterID)
				req.Equal("install", event.EntryCommand)
				req.Equal("--foo --bar", event.Flags)
				req.Equal(types.EventTypeInfraInstallationSucceeded, event.EventType)
				req.True(event.IsExitEvent)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(
					func(rw http.ResponseWriter, req *http.Request) {
						test.validateRequest(t, req)
						rw.Write([]byte(`OK`))
					},
				),
			)
			defer server.Close()

			reporter := NewReporter("test-execution-id", server.URL, "123", "install", []string{"--foo", "--bar"})
			reporter.ReportInstallationSucceeded(context.Background())
		})
	}
}
