package web

import (
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	// Create a test logger
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	// Create initial state
	initialState := InitialState{
		Title: "Test Title",
		Icon:  "test-icon.png",
	}

	// Create a new Web instance
	web := New(initialState, logger)

	// Verify the web instance was created correctly
	assert.Equal(t, initialState.Title, web.initialState.Title, "Title should match")
	assert.Equal(t, initialState.Icon, web.initialState.Icon, "Icon should match")
	assert.NotNil(t, web.logger, "Logger should be set")
}

func TestRootHandler(t *testing.T) {
	// Create a test logger
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	// Create initial state
	initialState := InitialState{
		Title: "Test Title",
		Icon:  "test-icon.png",
	}

	// Create a new Web instance
	web := New(initialState, logger)

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
}

func TestRootHandlerTemplateError(t *testing.T) {
	// Save the original template
	originalTemplate := htmlTemplate
	defer func() { htmlTemplate = originalTemplate }()

	// Create a template that will cause an error
	errorTemplate, err := template.New("error").Parse("{{.NonExistentField}}")
	assert.NoError(t, err, "Failed to parse error template")
	htmlTemplate = errorTemplate

	// Create a test logger
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	// Create initial state
	initialState := InitialState{
		Title: "Test Title",
		Icon:  "test-icon.png",
	}

	// Create a new Web instance
	web := New(initialState, logger)

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
	// Create a test logger
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	// Create initial state
	initialState := InitialState{
		Title: "Test Title",
		Icon:  "test-icon.png",
	}

	// Mock the assets file system
	iconContent := []byte("fake icon data")
	jsContent := []byte("console.log('Hello, world!');")
	mockFS := fstest.MapFS{
		"assets/" + initialState.Icon: &fstest.MapFile{
			Data: iconContent,
			Mode: 0644,
		},
		"assets/app.js": &fstest.MapFile{
			Data: jsContent,
			Mode: 0644,
		},
	}

	// Store the original assetsFS and restore it after the test
	originalAssetsFS := assetsFS
	assetsFS = mockFS
	defer func() { assetsFS = originalAssetsFS }()

	// Create a new Web instance
	web := New(initialState, logger)
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
		assert.Equal(t, string(iconContent), recorder.Body.String(), "Response should contain the icon content")
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
		assert.Equal(t, string(jsContent), recorder.Body.String(), "Response should contain the JS content")
	})
}
