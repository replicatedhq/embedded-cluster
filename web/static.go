package web

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

//go:embed dist
var static embed.FS

var embedAssetsFS fs.FS

func init() {
	var err error
	embedAssetsFS, err = fs.Sub(static, "dist")
	if err != nil {
		panic(err)
	}
}

type InitialState struct {
	Title string `json:"title"`
	Icon  string `json:"icon"`
}

type Web struct {
	// htmlTemplate is the parsed HTML template for the React app
	htmlTemplate *template.Template
	// assets is the filesystem containing static assets
	assets fs.FS
	// initialState is the initial state to be passed to the React app
	initialState InitialState
	logger       logrus.FieldLogger
}

type WebOption func(*Web)

func WithLogger(logger logrus.FieldLogger) WebOption {
	return func(web *Web) {
		web.logger = logger
	}
}

func WithAssetsFS(assets fs.FS) WebOption {
	return func(web *Web) {
		web.assets = assets
	}
}

// DefaultAssetsFS returns the default filesystem containing static assets
func DefaultAssetsFS() fs.FS {
	return embedAssetsFS
}

// New creates a new Web instance with the provided initial state and options
func New(initialState InitialState, opts ...WebOption) (*Web, error) {
	web := &Web{initialState: initialState}
	for _, opt := range opts {
		opt(web)
	}

	if web.logger == nil {
		web.logger = logrus.New().WithField("component", "web")
	}

	if web.assets == nil {
		web.assets = DefaultAssetsFS()
	}

	if web.htmlTemplate == nil {
		htmlTemplate, err := template.ParseFS(web.assets, "index.html")
		if err != nil {
			return nil, fmt.Errorf("failed to parse HTML template: %w", err)
		}
		web.htmlTemplate = htmlTemplate
	}

	return web, nil
}

func (web *Web) rootHandler(w http.ResponseWriter, r *http.Request) {
	stateJSON, err := json.Marshal(web.initialState)
	if err != nil {
		web.logger.WithError(err).
			Info("failed to marshal initial state")
		http.Error(w, "Error marshaling initial state", 500)
		return
	}

	data := struct {
		Title        string
		InitialState template.JS
	}{
		Title: web.initialState.Title,
		// State we're passing directly to the React app
		InitialState: template.JS(stateJSON), // Mark safe for unescaped JS
	}

	// Create a buffer to store the rendered template
	buf := new(bytes.Buffer)

	// Execute the template and write to the buffer
	err = web.htmlTemplate.Execute(buf, data)
	if err != nil {
		web.logger.WithError(err).
			Info("failed to execute HTML template")
		http.Error(w, "Template execution error", 500)
	}

	// Write the buffer contents to the response writer
	_, err = buf.WriteTo(w)
	if err != nil {
		web.logger.WithError(err).
			Info("failed to write response")
		return
	}
}

func (web *Web) RegisterRoutes(router *mux.Router) {
	var webFS http.Handler
	if os.Getenv("EC_DEV_ENV") == "true" {
		// Use absolute path to ensure we can find the files
		absPath, err := filepath.Abs("web/dist")
		if err != nil {
			web.logger.WithError(err).Error("Failed to get absolute path for assets")
		}
		webFS = http.FileServer(http.Dir(absPath))
	} else {
		webFS = http.FileServer(http.FS(web.assets))
	}

	// Create a custom handler that sets the correct MIME types
	assetsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Remove the nosniff header to allow content type override
		w.Header().Del("X-Content-Type-Options")

		// Set correct MIME types for different file types
		switch {
		case strings.HasSuffix(r.URL.Path, ".css"):
			w.Header().Set("Content-Type", "text/css")
		case strings.HasSuffix(r.URL.Path, ".js"):
			w.Header().Set("Content-Type", "application/javascript")
		case strings.HasSuffix(r.URL.Path, ".html"):
			w.Header().Set("Content-Type", "text/html")
		case strings.HasSuffix(r.URL.Path, ".json"):
			w.Header().Set("Content-Type", "application/json")
		}

		webFS.ServeHTTP(w, r)
	})

	router.PathPrefix("/assets").Methods("GET").Handler(assetsHandler)
	router.PathPrefix("/").Methods("GET").HandlerFunc(web.rootHandler)
}
