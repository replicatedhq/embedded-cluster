package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
)

var (
	builder     *release.Builder
	destination *release.Destination
)

// Response is the response body for the build endpoint.
type Response struct {
	URL string `json:"url"`
}

// healthz handles requests to /healthz endpoint. To be implemented.
func healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// build handles requests to /build endpoint.
func build(w http.ResponseWriter, r *http.Request) {
	req, err := release.BuildRequestFromHTTPRequest(r)
	if err != nil {
		logrus.Errorf("unable to parse and validate build request: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if url, err := destination.Exists(r.Context(), req); err != nil {
		logrus.Errorf("unable to check if version was already built: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if url != "" {
		if err := json.NewEncoder(w).Encode(Response{URL: url}); err != nil {
			logrus.Errorf("unable to encode response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
	release, err := builder.Build(r.Context(), req)
	if err != nil {
		logrus.Errorf("unable to build: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer release.Close()
	url, err := destination.Upload(r.Context(), req, release)
	if err != nil {
		logrus.Errorf("unable to upload: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(Response{URL: url}); err != nil {
		logrus.Errorf("unable to encode response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	logrus.Infof("build complete")
}

func main() {
	var err error
	logrus.Infof("staring embedded-cluster builder version %s", defaults.Version)
	if builder, err = release.NewBuilder(context.Background()); err != nil {
		logrus.Fatalf("unable to create builder: %v", err)
	}
	if destination, err = release.NewDestination(context.Background()); err != nil {
		logrus.Fatalf("unable to create destination: %v", err)
	}
	http.HandleFunc("/build", build)
	http.HandleFunc("/healthz", healthz)
	logrus.Infof("starting server at :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		logrus.Errorf("error starting server: %v", err)
	}
}
