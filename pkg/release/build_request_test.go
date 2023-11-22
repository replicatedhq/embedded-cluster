package release

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildRequestValidate(t *testing.T) {
	for _, tt := range []struct {
		name    string
		errMsg  string
		request BuildRequest
	}{
		{
			name: "Valid Request",
			request: BuildRequest{
				BinaryName:             "testBinary",
				EmbeddedClusterVersion: "v1.0.0",
				KOTSRelease:            "testRelease",
				KOTSReleaseVersion:     "v2.0.0",
				LicenseID:              "testLicenseID",
			},
		},
		{
			name:   "Missing BinaryName",
			errMsg: "binaryName is required",
			request: BuildRequest{
				EmbeddedClusterVersion: "v1.0.0",
				KOTSRelease:            "testRelease",
				KOTSReleaseVersion:     "v2.0.0",
				LicenseID:              "testLicenseID",
			},
		},
		{
			name:   "Missing EmbeddedClusterVersion",
			errMsg: "embeddedClusterVersion is required",
			request: BuildRequest{
				BinaryName:         "testBinary",
				KOTSRelease:        "testRelease",
				KOTSReleaseVersion: "v2.0.0",
				LicenseID:          "testLicenseID",
			},
		},
		{
			name:   "Missing KOTSRelease",
			errMsg: "kotsRelease is required",
			request: BuildRequest{
				EmbeddedClusterVersion: "v1.0.0",
				BinaryName:             "testBinary",
				KOTSReleaseVersion:     "v2.0.0",
				LicenseID:              "testLicenseID",
			},
		},
		{
			name:   "Missing KOTSReleaseVersion",
			errMsg: "kotsReleaseVersion is required",
			request: BuildRequest{
				EmbeddedClusterVersion: "v1.0.0",
				BinaryName:             "testBinary",
				KOTSRelease:            "testRelease",
				LicenseID:              "testLicenseID",
			},
		},
		{
			name:   "Missing LicenseID",
			errMsg: "licenseID is required",
			request: BuildRequest{
				EmbeddedClusterVersion: "v1.0.0",
				BinaryName:             "testBinary",
				KOTSRelease:            "testRelease",
				KOTSReleaseVersion:     "v2.0.0",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.errMsg == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.errMsg)
			}
		})
	}
}

func TestBuildRequest_uniqName(t *testing.T) {
	for _, tt := range []struct {
		name    string
		request BuildRequest
		want    string
	}{
		{
			name: "Valid UniqName",
			request: BuildRequest{
				BinaryName:             "binary",
				EmbeddedClusterVersion: "v1.0.0",
				KOTSReleaseVersion:     "v2.0.0",
				LicenseID:              "testLicenseID",
			},
			want: "binary-v1.0.0-v2.0.0-testLicenseID",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.request.uniqName()
			require.Equal(t, tt.want, result)
		})
	}
}

func TestBuildRequest_kotsReleaseReader(t *testing.T) {
	testString := "test data for kots release"
	encodedTestString := base64.StdEncoding.EncodeToString([]byte(testString))
	request := BuildRequest{KOTSRelease: encodedTestString}
	reader := request.kotsReleaseReader()
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, testString, string(data))
}

func TestBuildRequestFromHTTPRequest(t *testing.T) {
	validBuildRequest := BuildRequest{
		BinaryName:             "binary",
		EmbeddedClusterVersion: "1.0.0",
		KOTSRelease:            "release",
		KOTSReleaseVersion:     "2.0.0",
		LicenseID:              "licenseID",
	}
	reqBody, err := json.Marshal(validBuildRequest)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/build", bytes.NewReader(reqBody))
	_, err = BuildRequestFromHTTPRequest(req)
	require.NoError(t, err)
	req = httptest.NewRequest(http.MethodPost, "/build", strings.NewReader("!M<MSLR<>"))
	_, err = BuildRequestFromHTTPRequest(req)
	require.Error(t, err)
}
