package web

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"testing/fstest"
	"time"

	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

var appConstantsJsContent = `<script>
  var APP_TITLE = "%s";
  var APP_ICON_PATH = "%s";
  var APP_UPSTREAM_ICON_URI = "%s";
</script>`

var assetHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

// AssetsFs returns a fs.FS that contains a javascript file and the icon file itself.
// It is used to serve the application title, icon, and upstream icon URI to the web UI.
func AssetsFs(pathPrefix string) fs.FS {
	app := release.GetApplication()

	appTitle := getApplicationTitle(app)
	appUpstreamIconURI := getApplicationUpstreamIconURI(app)
	appIconData := getApplicationIconData(app)
	appIconFileName := getApplicationIconFileName(app)
	appIconPath := path.Join(pathPrefix, appIconFileName)

	return fstest.MapFS{
		path.Join(pathPrefix, "app-constants.js"): {
			Data:    fmt.Appendf([]byte{}, appConstantsJsContent, appTitle, appIconPath, appUpstreamIconURI),
			Mode:    0444,
			ModTime: time.Now(),
		},
		appIconPath: {
			Data:    appIconData,
			Mode:    0444,
			ModTime: time.Now(),
		},
	}
}

func getApplicationTitle(app *kotsv1beta1.Application) string {
	if app != nil {
		return app.Spec.Title
	}
	return "App"
}

func getApplicationUpstreamIconURI(app *kotsv1beta1.Application) string {
	if app != nil && app.Spec.Icon != "" {
		return app.Spec.Icon
	}
	return defaultIcon
}

func getApplicationIconData(app *kotsv1beta1.Application) []byte {
	if app != nil && app.Spec.Icon != "" {
		if data := getIconData(app.Spec.Icon); len(data) > 0 {
			return data
		}
	}
	return getIconData(defaultIcon)
}

func getApplicationIconFileName(app *kotsv1beta1.Application) string {
	if app != nil && app.Spec.Icon != "" {
		if icon := getIconFileName(app.Spec.Icon); icon != "" {
			return icon
		}
	}
	return getDefaultIconFileName()
}

func getDefaultIconFileName() string {
	return getIconFileName(defaultIcon)
}

func getIconData(icon string) []byte {
	// Check if it's a data URI (starts with data:)
	if strings.HasPrefix(icon, "data:") {
		// Extract base64 data
		if strings.Contains(icon, ";base64,") {
			dataParts := strings.Split(icon, ";base64,")
			if len(dataParts) > 1 {
				decoded, err := base64.StdEncoding.DecodeString(dataParts[1])
				if err == nil {
					return decoded
				}
			}
		}
	} else if strings.HasPrefix(icon, "http://") || strings.HasPrefix(icon, "https://") {
		// For URLs, try to fetch the icon
		resp, err := assetHTTPClient.Get(icon)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			if err == nil {
				return data
			}
		}
	}
	return nil
}

func getIconFileName(icon string) string {
	// Check if it's a data URI (starts with data:)
	if strings.HasPrefix(icon, "data:") {
		// Extract content type to determine file extension
		parts := strings.Split(icon, ";")
		if len(parts) > 0 {
			contentTypeParts := strings.Split(parts[0], ":")
			if len(contentTypeParts) > 1 {
				contentType := contentTypeParts[1]
				switch contentType {
				case "image/png":
					return "icon.png"
				case "image/jpeg", "image/jpg":
					return "icon.jpg"
				case "image/svg+xml":
					return "icon.svg"
				default:
					return ""
				}
			}
		}
	} else if strings.HasPrefix(icon, "http://") || strings.HasPrefix(icon, "https://") {
		// Try to determine file extension from URL
		urlParts := strings.Split(icon, ".")
		if len(urlParts) > 1 {
			ext := urlParts[len(urlParts)-1]
			if ext == "png" || ext == "jpg" || ext == "jpeg" || ext == "svg" {
				return "icon." + ext
			}
		}
	}
	return ""
}
