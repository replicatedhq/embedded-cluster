package web

import (
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

//go:embed dist
var static embed.FS

var assetsFS fs.FS
var htmlTemplate *template.Template

func init() {
	var err error
	assetsFS, err = fs.Sub(static, "dist")
	if err != nil {
		panic(err)
	}
	htmlTemplate, err = template.ParseFS(static, "dist/index.html")
	if err != nil {
		panic(err)
	}
}

type InitialState struct {
	Title string `json:"title"`
	Icon  string `json:"icon"`
}

type Web struct {
	initialState InitialState
	logger       logrus.FieldLogger
}

func New(initialState InitialState, logger logrus.FieldLogger) *Web {
	return &Web{
		initialState: initialState,
		logger:       logger,
	}
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

	err = htmlTemplate.Execute(w, data)
	if err != nil {
		web.logger.WithError(err).
			Info("failed to execute HTML template")
		http.Error(w, "Template execution error", 500)
	}
}

func (web *Web) RegisterRoutes(router *mux.Router) {

	var webFS http.Handler
	if os.Getenv("EC_DEV_ENV") == "true" {
		webFS = http.FileServer(http.FS(os.DirFS("./web/dist/assets")))
	} else {
		webFS = http.FileServer(http.FS(assetsFS))
	}

	router.PathPrefix("/assets").Methods("GET").Handler(webFS)
	router.PathPrefix("/").Methods("GET").HandlerFunc(web.rootHandler)
}
