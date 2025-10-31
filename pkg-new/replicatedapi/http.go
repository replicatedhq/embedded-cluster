package replicatedapi

import (
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// newRetryableHTTPClient returns a new retryablehttp.Client with default settings.
func newRetryableHTTPClient() *retryablehttp.Client {
	client := retryablehttp.NewClient()
	client.Logger = nil
	// errorHandler mimics net/http rather than doing anything fancy like the retryablehttp library.
	client.ErrorHandler = func(resp *http.Response, err error, attempt int) (*http.Response, error) {
		return resp, err
	}

	// Set the timeout to 30 seconds.
	client.HTTPClient.Timeout = 30 * time.Second
	return client
}
