package metrics

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/metrics/types"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
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
			OSArgs: []string{"secret-command", "--license", "./path", "--password", "123", "--some-key", "some-value", "--another-secret", "secret"},
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
				req.Equal("secret-command --license ./path --password ***** --some-key ***** --another-secret *****", event.Flags)
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
			license := &kotsv1beta1.License{
				Spec: kotsv1beta1.LicenseSpec{
					LicenseID: "license-id",
					AppSlug:   "app-slug",
					Endpoint:  server.URL,
				},
			}
			// Report call relies on os.Args to get the command and flags used so we nee to mock it
			originalArgs := os.Args
			defer func() { os.Args = originalArgs }()
			os.Args = append([]string{os.Args[0]}, test.OSArgs...)

			ReportInstallationStarted(context.Background(), license)
		})
	}
}
