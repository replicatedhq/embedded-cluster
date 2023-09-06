package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"

	"github.com/replicatedhq/helmvm/pkg/goods"
	"github.com/replicatedhq/helmvm/pkg/hembed"
)

// build handles requests to /build endpoint.
func build(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := req.FromHTTPRequest(r); err != nil {
		logrus.Errorf("unable to decode request: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	logrus.Infof("building custom helmvm binary")
	opts := hembed.EmbedOptions{
		OS:     req.OS,
		Arch:   req.Arch,
		Images: req.Images,
		Charts: req.Charts,
	}
	if err := hembed.ValidateTarget(opts); err != nil {
		logrus.Errorf("invalid target: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	bin := hembed.PathToPrebuiltBinary(opts)
	from, err := hembed.Embed(r.Context(), bin, opts)
	if err != nil {
		logrus.Errorf("unable to build helmvm: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer from.Close()
	w.Header().Set("Content-Disposition", "attachment; filename="+req.Name)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", from.Size()))
	w.Header().Set("Content-Type", "application/octet-stream")
	if _, err := io.Copy(w, from); err != nil {
		logrus.Errorf("unable to write response: %v", err)
		return
	}
	logrus.Infof("custom helmvm successfully built")
}

func download(w http.ResponseWriter, r *http.Request) {
	content, err := goods.WebFile("download.sh")
	if err != nil {
		logrus.Errorf("unable to read download script: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	content = bytes.ReplaceAll(content, []byte("{HOST}"), []byte(r.Host))
	content = bytes.ReplaceAll(content, []byte("{SCHEME}"), []byte(scheme))
	_, _ = w.Write(content)
}

func main() {
	logrus.Infof("staring helmvm build server")
	http.HandleFunc("/build", build)
	http.HandleFunc("/", download)
	logrus.Infof("starting server at :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		logrus.Errorf("error starting server: %v", err)
	}
}
