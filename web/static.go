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

// InitialState holds the initial information provided to the web server to be injected in the HTML doc rendered
type InitialState struct {
	Title string `json:"title"`
	Icon  string `json:"icon"`
	// The target of the installation, kubernetes or linux
	InstallTarget string `json:"installTarget"`
	// The mode the web will be running on, install or upgrade
	Mode Mode `json:"mode"`
	// Whether this is an airgap installation/upgrade
	IsAirgap bool `json:"isAirgap"`
	// Whether an infrastructure upgrade is required (only relevant for upgrades)
	RequiresInfraUpgrade bool `json:"requiresInfraUpgrade"`
}

type Mode string

const (
	ModeUpgrade Mode = "upgrade"
	ModeInstall Mode = "install"
)

type Web struct {
	// htmlTemplate is the parsed HTML template for the React app
	htmlTemplate *template.Template
	// assets is the filesystem containing static assets
	assets fs.FS
	// initialState is the initial state to be passed to the React app
	initialState InitialState
	// wheter we're running in dev mode or not
	isDev  bool
	logger logrus.FieldLogger
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

	// TODO we might consider moving this env var evaluation to the CLI and make it an overarching property of the project
	if os.Getenv("EC_DEV_ENV") == "true" || os.Getenv("EC_DEV_ENV") == "1" {
		web.isDev = true
	}

	if web.logger == nil {
		web.logger = logrus.New().WithField("component", "web")
	}

	if web.assets == nil {
		// By default, when running in dev mode we want to dinamically read our assets from source
		if web.isDev {
			web.assets = os.DirFS("./web/dist")
		} else {
			web.assets = DefaultAssetsFS()
		}
	}

	if web.htmlTemplate == nil {
		err := web.loadHTMLTemplate()
		if err != nil {
			return nil, err
		}
	}

	return web, nil
}

// loadHTMLTemplate parses the `index.html` file from the `web.assets` FS and stores it in the struct's `htmlTemplate`
// property
func (web *Web) loadHTMLTemplate() error {
	htmlTemplate, err := template.ParseFS(web.assets, "index.html")
	if err != nil {
		return fmt.Errorf("failed to parse HTML template: %w", err)
	}
	web.htmlTemplate = htmlTemplate
	return nil
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

	// When running in dev mode we need to parse the HTML template on every requeest since the JS build might have
	// generated a new index.html and set of assets.
	if web.isDev {
		web.loadHTMLTemplate()
	}

	// Execute the template and write to the buffer
	err = web.htmlTemplate.Execute(buf, data)
	if err != nil {
		web.logger.WithError(err).
			Info("failed to execute HTML template")
		http.Error(w, "Template execution error", 500)
		return
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
	webFS := http.FileServer(http.FS(web.assets))
	router.PathPrefix("/assets").Methods("GET").Handler(webFS)
	router.PathPrefix("/").Methods("GET").HandlerFunc(web.rootHandler)
}
