package logger

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// responseWriter wraps http.ResponseWriter to capture status code and response size
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

// WriteHeader captures the status code before writing it
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures the number of bytes written
func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// HTTPLoggingMiddleware creates a middleware that logs HTTP requests with structured fields.
// It logs after the request is handled, capturing the response status code and duration.
func HTTPLoggingMiddleware(logger logrus.FieldLogger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap the response writer to capture status code and bytes written
			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK, // default if WriteHeader is not called
			}

			// Call the next handler
			next.ServeHTTP(wrapped, r)

			// Log after the request is handled
			duration := time.Since(start)

			// Build structured log fields
			fields := logrus.Fields{
				"component":      "request-logger",
				"method":         r.Method,
				"path":           r.URL.Path,
				"status":         wrapped.statusCode,
				"duration_ms":    duration.Milliseconds(),
				"response_bytes": wrapped.written,
			}

			// Add query parameters if present
			if r.URL.RawQuery != "" {
				fields["query"] = r.URL.RawQuery
			}

			// Determine log level based on status code
			entry := logger.WithFields(fields)
			switch {
			case wrapped.statusCode >= 500:
				entry.Error("HTTP request")
			case wrapped.statusCode >= 400:
				entry.Warn("HTTP request")
			default:
				entry.Info("HTTP request")
			}
		})
	}
}
