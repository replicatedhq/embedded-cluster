package web

import (
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
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

func Fs() fs.FS {
	return assetsFS
}

func RootHandler(w http.ResponseWriter, r *http.Request) {
	state := map[string]any{
		"app-name":         "My App",
		"theme":            "dark",
		"some-other-state": "example value",
	}
	stateJSON, _ := json.Marshal(state)

	data := struct {
		Title        string
		Favicon      string
		InitialState template.JS
	}{
		Title:        "My App - Home",
		Favicon:      "/assets/favicon.png",
		InitialState: template.JS(stateJSON), // Inject unescaped JS
	}

	err := htmlTemplate.Execute(w, data)
	if err != nil {
		http.Error(w, "Template execution error", 500)
	}
}
