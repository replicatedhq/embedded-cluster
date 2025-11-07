package install

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_orchestrator_configureApplication(t *testing.T) {
	tests := []struct {
		name                     string
		mockPatchFunc            func(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfigValues, error)
		configValues             apitypes.AppConfigValues
		expectError              bool
		expectedErrorMsg         string
		expectedLogMessages      []string
		expectedProgressMessages []string
	}{
		{
			name: "success",
			mockPatchFunc: func(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfigValues, error) {
				return values, nil
			},
			configValues: apitypes.AppConfigValues{
				"hostname": apitypes.AppConfigValue{
					Value: "test.example.com",
				},
			},
			expectError:         false,
			expectedLogMessages: []string{},
			expectedProgressMessages: []string{
				"Configuring application...",
				"Application configuration complete",
			},
		},
		{
			name: "validation errors",
			mockPatchFunc: func(ctx context.Context, values apitypes.AppConfigValues) (apitypes.AppConfigValues, error) {
				return nil, &apitypes.APIError{
					StatusCode: 400,
					Message:    "field errors",
					Errors: []*apitypes.APIError{
						{
							Field:   "database_host",
							Message: "required field missing",
						},
						{
							Field:   "replica_count",
							Message: "value \"10\" exceeds maximum allowed value 5",
						},
						{
							Field:   "enable_ssl",
							Message: "validation rule failed: SSL requires cert_path to be set",
						},
					},
				}
			},
			configValues: apitypes.AppConfigValues{
				"database_host": apitypes.AppConfigValue{
					Value: "",
				},
				"replica_count": apitypes.AppConfigValue{
					Value: "10",
				},
			},
			expectError:         true,
			expectedErrorMsg:    "application configuration validation failed: field errors:\n  - Field 'database_host': required field missing\n  - Field 'replica_count': value \"10\" exceeds maximum allowed value 5\n  - Field 'enable_ssl': validation rule failed: SSL requires cert_path to be set",
			expectedLogMessages: []string{},
			expectedProgressMessages: []string{
				"Configuring application...",
				"Application configuration failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockClient := &mockAPIClient{
				patchLinuxInstallAppConfigValuesFunc: tt.mockPatchFunc,
			}

			// Create logger with capture hook
			logger := logrus.New()
			logger.SetOutput(io.Discard) // Discard actual output, we only want the hook
			logCapture := newLogMessageCapture()
			logger.AddHook(logCapture)

			// Capture progress writer messages
			progressCapture := newProgressMessageCapture()

			orchestrator := &orchestrator{
				apiClient:      mockClient,
				target:         apitypes.InstallTargetLinux,
				progressWriter: progressCapture.Writer(),
				logger:         logger,
			}

			opts := HeadlessInstallOptions{
				ConfigValues: tt.configValues,
			}

			// Execute
			err := orchestrator.configureApplication(context.Background(), opts)

			// Assert error
			if tt.expectError {
				require.Error(t, err)
				if tt.expectedErrorMsg != "" {
					assert.Equal(t, tt.expectedErrorMsg, err.Error())
				}
			} else {
				require.NoError(t, err)
			}

			// Assert log messages
			assert.Equal(t, tt.expectedLogMessages, logCapture.Messages(), "log messages should match")

			// Assert progress messages
			assert.Equal(t, tt.expectedProgressMessages, progressCapture.Messages(), "progress messages should match")
		})
	}
}

// logMessageCapture is a logrus hook that captures log messages for testing.
// It implements the logrus.Hook interface to intercept all log messages and
// store them for verification in tests. This allows tests to verify that
// specific log messages are produced without requiring actual log output.
type logMessageCapture struct {
	messages []string
}

// newLogMessageCapture creates a new log message capture helper.
// Usage:
//
//	logger := logrus.New()
//	capture := newLogMessageCapture()
//	logger.AddHook(capture)
//	// ... perform operations that log ...
//	assert.Equal(t, expectedMessages, capture.Messages())
func newLogMessageCapture() *logMessageCapture {
	return &logMessageCapture{
		messages: make([]string, 0),
	}
}

// Levels returns the log levels this hook applies to (all levels)
func (l *logMessageCapture) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire is called when a log event occurs
func (l *logMessageCapture) Fire(entry *logrus.Entry) error {
	msg := entry.Message
	if msg != "" {
		l.messages = append(l.messages, msg)
	}
	return nil
}

// Messages returns all captured log messages
func (l *logMessageCapture) Messages() []string {
	return l.messages
}

// progressMessageCapture captures progress messages from a spinner.WriteFn for testing.
// It parses spinner output (which includes ANSI codes and symbols like ○, ✔, ✗) and
// extracts just the message text. This allows tests to verify user-visible progress
// messages without dealing with terminal formatting codes.
type progressMessageCapture struct {
	messages    []string
	lastMessage string
}

// newProgressMessageCapture creates a new progress message capture helper.
// Usage:
//
//	capture := newProgressMessageCapture()
//	orchestrator := &orchestrator{
//	    progressWriter: capture.Writer(),
//	}
//	// ... perform operations that display progress ...
//	assert.Equal(t, expectedMessages, capture.Messages())
func newProgressMessageCapture() *progressMessageCapture {
	return &progressMessageCapture{
		messages: make([]string, 0),
	}
}

// Writer returns a spinner.WriteFn that captures progress messages.
// The returned function strips ANSI escape codes and spinner symbols (○, ✔, ✗)
// to extract just the message text. It deduplicates consecutive identical messages
// to avoid capturing spinner animation frames.
func (p *progressMessageCapture) Writer() spinner.WriteFn {
	return func(format string, args ...any) (int, error) {
		// Remove ANSI escape codes
		cleanFormat := strings.ReplaceAll(format, "\033[K\r", "")

		// Format the string with arguments
		var formatted string
		if len(args) > 0 {
			formatted = strings.TrimSpace(fmt.Sprintf(cleanFormat, args...))
		} else {
			formatted = strings.TrimSpace(cleanFormat)
		}

		// Extract just the message part (skip spinner/checkmark symbols)
		// Format is: "○  message" or "✔  message" or "✗  message"
		parts := strings.SplitN(formatted, "  ", 2)
		if len(parts) == 2 {
			msg := strings.TrimSpace(parts[1])
			// Only add if message changed and it's not empty
			if msg != p.lastMessage && msg != "" {
				p.messages = append(p.messages, msg)
				p.lastMessage = msg
			}
		}
		return 0, nil
	}
}

// Messages returns all captured progress messages
func (p *progressMessageCapture) Messages() []string {
	return p.messages
}
