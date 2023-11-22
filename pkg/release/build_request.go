package release

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// BuildRequest represents what needs to be built.
type BuildRequest struct {
	BinaryName             string `json:"binaryName"`
	EmbeddedClusterVersion string `json:"embeddedClusterVersion"`
	LicenseID              string `json:"licenseID"`
	KOTSRelease            string `json:"kotsRelease"`
	KOTSReleaseVersion     string `json:"kotsReleaseVersion"`
}

// Validate makes sure the build request is valid.
func (b *BuildRequest) Validate() error {
	if b.BinaryName == "" {
		return fmt.Errorf("binaryName is required")
	}
	if b.EmbeddedClusterVersion == "" {
		return fmt.Errorf("embeddedClusterVersion is required")
	}
	if b.KOTSRelease == "" {
		return fmt.Errorf("kotsRelease is required")
	}
	if b.KOTSReleaseVersion == "" {
		return fmt.Errorf("kotsReleaseVersion is required")
	}
	if b.LicenseID == "" {
		return fmt.Errorf("licenseID is required")
	}
	return nil
}

// UniqName returns a name that uniquely identifies a release build.
func (b *BuildRequest) uniqName() string {
	return fmt.Sprintf(
		"%s-%s-%s-%s",
		b.BinaryName,
		b.EmbeddedClusterVersion,
		b.KOTSReleaseVersion,
		b.LicenseID,
	)
}

// kotsReleaseReader returns a reader for the kots release. KOTS Release property is
// a string of a base64 encoded tar.gz file.
func (b *BuildRequest) kotsReleaseReader() io.Reader {
	return base64.NewDecoder(base64.StdEncoding, strings.NewReader(b.KOTSRelease))
}

// BuildRequestFromHTTPRequest parses a build request from an http request.
func BuildRequestFromHTTPRequest(r *http.Request) (*BuildRequest, error) {
	req := &BuildRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return nil, fmt.Errorf("unable to decode request: %v", err)
	}
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %v", err)
	}
	return req, nil
}
