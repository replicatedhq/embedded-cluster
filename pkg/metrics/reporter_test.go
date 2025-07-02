package metrics

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
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
			OSArgs: []string{"install", "-l", "./license.yaml", "--admin-console-password", "some-password", "--skip-host-preflights", "--http-proxy", "http://user:password@my-proxy.test", "--https-proxy=https://user:password@my-https-proxy.test", "--admin-console-port", "8080"},
			validateRequest: func(t *testing.T, r *http.Request) {
				req := require.New(t)
				body, err := io.ReadAll(r.Body)
				req.NoError(err)
				var decoded map[string]json.RawMessage
				var event types.InstallationStarted
				err = json.Unmarshal(body, &decoded)
				req.NoError(err)
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
			// Report call relies on os.Args to get the command and flags used so we nee to mock it
			originalArgs := os.Args
			defer func() { os.Args = originalArgs }()
			os.Args = append([]string{os.Args[0]}, test.OSArgs...)

			reporter := NewReporter("test-execution-id", server.URL, uuid.New(), "install", test.OSArgs[1:])
			reporter.ReportInstallationStarted(context.Background(), "license-id", "app-slug")
		})
	}
}
