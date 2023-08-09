package main

import (
	"net/http"

	"gopkg.in/yaml.v2"

	"github.com/replicatedhq/helmvm/pkg/hembed"
)

// Request is the request body for the build endpoint. Everything here
// is optional.
type Request struct {
	Name   string             `yaml:"name"`
	Arch   string             `yaml:"arch"`
	OS     string             `yaml:"os"`
	Images []string           `yaml:"images"`
	Charts []hembed.HelmChart `yaml:"charts"`
}

// defaults sets the default values for fields that are not defined.
func (r *Request) defaults() {
	if r.Name == "" {
		r.Name = "helmvm"
	}
	if r.OS == "" {
		r.OS = "linux"
	}
	if r.Arch == "" {
		r.Arch = "amd64"
	}
	if r.Charts == nil {
		r.Charts = []hembed.HelmChart{}
	}
}

// FromHTTPRequest decodes a Request object from a HTTP request.
func (r *Request) FromHTTPRequest(hreq *http.Request) error {
	if hreq.Method == http.MethodGet {
		r.Name = hreq.URL.Query().Get("name")
		r.OS = hreq.URL.Query().Get("os")
		r.Arch = hreq.URL.Query().Get("arch")
		r.defaults()
		return nil
	}
	if err := yaml.NewDecoder(hreq.Body).Decode(r); err != nil {
		return err
	}
	r.defaults()
	return nil
}
