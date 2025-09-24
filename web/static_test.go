package web

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"runtime"
	"testing"
	"testing/fstest"

	"github.com/gorilla/mux"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overrideDefaultFS temporarily replaces the assetsFS with a mock filesystem and returns a cleanup function
func overrideDefaultFS(mockFS fstest.MapFS) func() {
	// Store the original assetsFS
	originalAssetsFS := embedAssetsFS

	// Replace with mock filesystem
	embedAssetsFS = mockFS

	// Return cleanup function
	return func() {
		embedAssetsFS = originalAssetsFS
	}
}

var htmlTemplate = []byte(`<!DOCTYPE html>
				<html>
				<head>
					<title>{{.Title}}</title>
				</head>
				<body>
					<script>
						window.__INITIAL_STATE__ = {{.InitialState}};
					</script>
				</body>
				</html>`,
)

// createMockFS creates a standard mock filesystem for testing
func createMockFS() fstest.MapFS {
	// Create mock assets
	return fstest.MapFS{
		"assets/test-icon.png": &fstest.MapFile{
			Data: []byte("fake icon data"),
			Mode: 0644,
		},
		"assets/app.js": &fstest.MapFile{
			Data: []byte("console.log('Hello, world!');"),
			Mode: 0644,
		},
		"index.html": &fstest.MapFile{
			Data: htmlTemplate,
			Mode: 0644,
		},
	}
}

func TestNew(t *testing.T) {
	// Create initial state
	initialState := InitialState{
		Title: "Test Title",
		Icon:  "test-icon.png",
	}

	// Create a test logger
	logger, _ := logtest.NewNullLogger()

	// Set the development environment variable to false
	t.Setenv("EC_DEV_ENV", "false")

	// Create a new Web instance
	web, err := New(initialState, WithLogger(logger), WithAssetsFS(createMockFS()))
	require.NoError(t, err, "Failed to create Web instance")

	// Verify the web instance was created correctly
	assert.Equal(t, initialState.Title, web.initialState.Title, "Title should match")
	assert.Equal(t, initialState.Icon, web.initialState.Icon, "Icon should match")
	assert.NotNil(t, web.logger, "Logger should be set")
	assert.NotNil(t, web.htmlTemplate, "HTML template should be set")
}

func TestNewWithDefaultFS(t *testing.T) {
	// Override the default filesystem with a mock one
	cleanup := overrideDefaultFS(createMockFS())
	defer cleanup()

	// Create initial state
	initialState := InitialState{
		Title: "Test Title",
		Icon:  "test-icon.png",
	}

	// Create a test logger
	logger, _ := logtest.NewNullLogger()

	// Set the development environment variable to false
	t.Setenv("EC_DEV_ENV", "false")

	// Create a new Web instance
	web, err := New(initialState, WithLogger(logger))
	require.NoError(t, err, "Failed to create Web instance")

	// Verify the web instance was created correctly
	assert.Equal(t, initialState.Title, web.initialState.Title, "Title should match")
	assert.Equal(t, initialState.Icon, web.initialState.Icon, "Icon should match")
	assert.NotNil(t, web.logger, "Logger should be set")
	assert.NotNil(t, web.htmlTemplate, "HTML template should be set")
}

// TestNewWithIndexHTML tests creating a Web instance with the actual index.html template we use and pass over to Vite for building.
func TestNewWithIndexHTML(t *testing.T) {
	// Setup a mock filesystem with our actual index.html file
	indexHTML, err := os.ReadFile("index.html")
	require.NoError(t, err, "Failed to read index.html")

	mockFS := fstest.MapFS{
		"assets/test-icon.png": &fstest.MapFile{
			Data: []byte("fake icon data"),
			Mode: 0644,
		},
		"assets/app.js": &fstest.MapFile{
			Data: []byte("console.log('Hello, world!');"),
			Mode: 0644,
		},
		"index.html": &fstest.MapFile{
			Data: indexHTML,
			Mode: 0644,
		},
	}

	// Create initial state
	initialState := InitialState{
		Title: "Test Title",
		Icon:  "test-icon.png",
	}

	// Create a test logger
	logger, _ := logtest.NewNullLogger()

	// Set the development environment variable to false
	t.Setenv("EC_DEV_ENV", "false")

	// Create a new Web instance, using the actual index.html template
	web, err := New(initialState, WithLogger(logger), WithAssetsFS(mockFS))
	require.NoError(t, err, "Failed to create Web instance")

	// Verify the web instance was created correctly
	assert.Equal(t, initialState.Title, web.initialState.Title, "Title should match")
	assert.Equal(t, initialState.Icon, web.initialState.Icon, "Icon should match")
}

func TestNewWithNonExistentTemplate(t *testing.T) {
	// Setup a mock filesystem without an index.html file
	mockFS := fstest.MapFS{
		"assets/test-icon.png": &fstest.MapFile{
			Data: []byte("fake icon data"),
			Mode: 0644,
		},
		"assets/app.js": &fstest.MapFile{
			Data: []byte("console.log('Hello, world!');"),
			Mode: 0644,
		},
		// Deliberately omitting index.html
	}

	// Create initial state
	initialState := InitialState{
		Title: "Test Title",
		Icon:  "test-icon.png",
	}

	// Create a test logger
	logger, _ := logtest.NewNullLogger()

	// Set the development environment variable to false
	t.Setenv("EC_DEV_ENV", "false")

	// Try to create a new Web instance without providing an HTML template
	web, err := New(initialState, WithLogger(logger), WithAssetsFS(mockFS))

	// Assert that an error was returned
	assert.Error(t, err, "New should return an error when the template doesn't exist")

	// Assert that the error message mentions the template
	assert.Contains(t, err.Error(), "failed to parse HTML template", "Error should mention template parsing failure")

	// Assert that the web instance is nil
	assert.Nil(t, web, "Web instance should be nil when an error occurs")
}

func TestRootHandler(t *testing.T) {
	initialState := InitialState{
		Title:         "Test Title",
		Icon:          "test-icon.png",
		InstallTarget: "linux",
		Mode:          "install",
	}

	// Create a test logger
	logger, _ := logtest.NewNullLogger()

	// Set the development environment variable to false
	t.Setenv("EC_DEV_ENV", "false")

	// Create a new Web instance
	web, err := New(initialState, WithLogger(logger), WithAssetsFS(createMockFS()))
	require.NoError(t, err, "Failed to create Web instance")

	// Create a mock HTTP request
	req := httptest.NewRequest("GET", "/", nil)
	recorder := httptest.NewRecorder()

	// Call the rootHandler
	web.rootHandler(recorder, req)

	// Check status code
	assert.Equal(t, http.StatusOK, recorder.Code, "Should return status OK")

	// Read the response body
	body := recorder.Body.String()

	// Check that the title is in the response
	assert.Contains(t, body, initialState.Title, "Response should contain the title")

	// Check that the initial state JSON is in the response
	stateJSON, _ := json.Marshal(initialState)
	assert.Contains(t, body, string(stateJSON), "Response should contain initial state JSON")

	// Check that specific fields are in the JSON
	assert.Contains(t, body, `"installTarget":"linux"`, "Response should contain install target")
	assert.Contains(t, body, `"mode":"install"`, "Response should contain mode")
}

func TestRootHandlerTemplateError(t *testing.T) {
	// Create initial state
	initialState := InitialState{
		Title: "Test Title",
		Icon:  "test-icon.png",
	}

	// Create a test logger
	logger, _ := logtest.NewNullLogger()

	// Set the development environment variable to false
	t.Setenv("EC_DEV_ENV", "false")

	// Create a new Web instance
	web, err := New(initialState, WithLogger(logger), WithAssetsFS(createMockFS()))
	require.NoError(t, err, "Failed to create Web instance")

	// Replace the template with one that will cause an error
	errorTemplate, err := template.New("error").Parse("{{.NonExistentField}}")
	assert.NoError(t, err, "Failed to parse error template")

	// Save original and replace with error template
	originalTemplate := web.htmlTemplate
	web.htmlTemplate = errorTemplate
	defer func() { web.htmlTemplate = originalTemplate }()

	// Create a mock HTTP request
	req := httptest.NewRequest("GET", "/", nil)
	recorder := httptest.NewRecorder()

	// Call the rootHandler
	web.rootHandler(recorder, req)

	// Check status code
	assert.Equal(t, http.StatusInternalServerError, recorder.Code, "Should return internal server error")

	// Check that the error message is in the response
	expectedError := "Template execution error"
	assert.Contains(t, recorder.Body.String(), expectedError, "Response should contain error message")
}

func TestRegisterRoutes(t *testing.T) {
	// Create initial state
	initialState := InitialState{
		Title: "Test Title",
		Icon:  "test-icon.png",
	}

	// Create a test logger
	logger, _ := logtest.NewNullLogger()

	// Set the development environment variable to false
	t.Setenv("EC_DEV_ENV", "false")

	// Create a new Web instance
	web, err := New(initialState, WithLogger(logger), WithAssetsFS(createMockFS()))
	require.NoError(t, err, "Failed to create Web instance")

	// Create router
	router := mux.NewRouter()
	web.RegisterRoutes(router)

	t.Run("Root Path", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		recorder := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(recorder, req)

		// Check status code
		assert.Equal(t, http.StatusOK, recorder.Code, "Should return status OK")

		// Check that the title is in the response
		assert.Contains(t, recorder.Body.String(), initialState.Title, "Response should contain the title")
	})

	// Test 2: Icon asset
	t.Run("Icon Asset", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/assets/"+initialState.Icon, nil)
		recorder := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(recorder, req)

		// Check status code
		assert.Equal(t, http.StatusOK, recorder.Code, "Should return status OK for icon")

		// Check that the icon content is in the response
		assert.Equal(t, "fake icon data", recorder.Body.String(), "Response should contain the icon content")
	})

	// Test 3: JS asset
	t.Run("JS Asset", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/assets/app.js", nil)
		recorder := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(recorder, req)

		// Check status code
		assert.Equal(t, http.StatusOK, recorder.Code, "Should return status OK for JS file")

		// Check that the JS content is in the response
		assert.Equal(t, "console.log('Hello, world!');", recorder.Body.String(), "Response should contain the JS content")
	})
}
func TestRegisterRoutesWithDevEnv(t *testing.T) {
	// We need to change the current working directory because in `go test` this will be the package directory
	// We want to mimic prod/local dev behaviour where cwd will be under the root of the project
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Join(path.Dir(filename), "..")
	err := os.Chdir(dir)
	if err != nil {
		t.Fatalf("failed to change cwd to root of the project: %s", err)
	}
	defer os.Chdir(path.Dir(filename))

	// Create a test logger
	logger, _ := logtest.NewNullLogger()
	// Set the development environment variable
	t.Setenv("EC_DEV_ENV", "true")

	// Create temporary dist directory structure to mimic what we use for development
	err = os.MkdirAll("./web/dist/assets", 0755)
	require.NoError(t, err, "Failed to create dist directory")
	defer os.RemoveAll("./web/dist/assets") // Clean up after test

	// Create a test file in the dist/assets directory
	devFileContent := "console.log('Development mode!');"
	err = os.WriteFile("./web/dist/assets/test-file-dev-app.js", []byte(devFileContent), 0644)
	require.NoError(t, err, "Failed to write dev file")

	// Create a index.hmtl test file in the dist/ directory to be used as template by the web server
	err = os.WriteFile("./web/dist/index.html", []byte(htmlTemplate), 0644)
	require.NoError(t, err, "Failed to write dev file")

	// Create a new Web instance
	web, err := New(InitialState{}, WithLogger(logger))
	require.NoError(t, err, "Failed to create Web instance")

	// Create router and register routes
	router := mux.NewRouter()
	web.RegisterRoutes(router)

	t.Run("Dev File from Local Filesystem", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/assets/test-file-dev-app.js", nil)
		recorder := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(recorder, req)

		// Check status code
		assert.Equal(t, http.StatusOK, recorder.Code, "Should return status OK for dev file")

		// Check that the dev file content is served from local filesystem
		assert.Equal(t, devFileContent, recorder.Body.String(), "Response should contain the dev file content from local filesystem")
	})

	t.Run("Changes to the file are reflected and served", func(t *testing.T) {
		newDevFileContent := devFileContent + "console.log('such a change, very wow');"
		err = os.WriteFile("./web/dist/assets/test-file-dev-app.js", []byte(newDevFileContent), 0644)
		req := httptest.NewRequest("GET", "/assets/test-file-dev-app.js", nil)
		recorder := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(recorder, req)

		// Check status code
		assert.Equal(t, http.StatusOK, recorder.Code, "Should return status OK for dev file")

		// Check that the new dev file content is served from local filesystem
		assert.Equal(t, newDevFileContent, recorder.Body.String(), "Response should contain the dev file content from local filesystem")
	})

	t.Run("Non-existent File Returns 404", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/assets/non-existent.js", nil)
		recorder := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(recorder, req)

		// Check status code - should be 404 when file doesn't exist in local filesystem
		assert.Equal(t, http.StatusNotFound, recorder.Code, "Should return 404 for non-existent file in dev mode")
	})
}

func TestRootHandlerWithDifferentModes(t *testing.T) {
	tests := []struct {
		name          string
		installTarget string
		mode          string
		title         string
	}{
		{"linux install", "linux", "install", "Linux Install App"},
		{"linux upgrade", "linux", "upgrade", "Linux Upgrade App"},
		{"kubernetes install", "kubernetes", "install", "K8s Install App"},
		{"kubernetes upgrade", "kubernetes", "upgrade", "K8s Upgrade App"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialState := InitialState{
				Title:         tt.title,
				Icon:          "test-icon.png",
				InstallTarget: tt.installTarget,
				Mode:          tt.mode,
			}

			// Create a test logger
			logger, _ := logtest.NewNullLogger()

			// Set the development environment variable to false
			t.Setenv("EC_DEV_ENV", "false")

			// Create a new Web instance
			web, err := New(initialState, WithLogger(logger), WithAssetsFS(createMockFS()))
			require.NoError(t, err, "Failed to create Web instance")

			// Create a mock HTTP request
			req := httptest.NewRequest("GET", "/", nil)
			recorder := httptest.NewRecorder()

			// Call the rootHandler
			web.rootHandler(recorder, req)

			// Check status code
			assert.Equal(t, http.StatusOK, recorder.Code, "Should return status OK")

			// Read the response body
			body := recorder.Body.String()

			// Verify the complete JSON is valid
			stateJSON, _ := json.Marshal(initialState)
			assert.Contains(t, body, string(stateJSON), "Response should contain initial state JSON")
		})
	}
}
