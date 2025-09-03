package helm

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_binaryExecutor_ExecuteCommand(t *testing.T) {
	tests := []struct {
		name    string
		bin     string
		args    []string
		wantErr bool
	}{
		{
			name:    "echo command",
			bin:     "echo",
			args:    []string{"hello", "world"},
			wantErr: false,
		},
		{
			name:    "invalid command",
			bin:     "nonexistent-command",
			args:    []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := newBinaryExecutor(tt.bin)
			stdout, stderr, err := executor.ExecuteCommand(t.Context(), nil, nil, tt.args...)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Empty(t, stderr)
			if tt.bin == "echo" {
				assert.Contains(t, stdout, "hello world")
			}
		})
	}
}

func Test_binaryExecutor_ExecuteCommand_WithLogging(t *testing.T) {
	tests := []struct {
		name           string
		bin            string
		args           []string
		wantErr        bool
		expectedStdout string
		expectedStderr string
		expectedLogs   []string
	}{
		{
			name:           "echo command with logging",
			bin:            "echo",
			args:           []string{"hello", "world"},
			wantErr:        false,
			expectedStdout: "hello world\n",
			expectedStderr: "",
			expectedLogs:   []string{}, // No logs expected since echo only writes to stdout
		},
		{
			name:           "command with stderr",
			bin:            "sh",
			args:           []string{"-c", "echo 'stdout message'; echo 'stderr message' >&2"},
			wantErr:        false,
			expectedStdout: "stdout message\n",
			expectedStderr: "stderr message\n",
			expectedLogs:   []string{}, // No logs expected since stderr doesn't match .go file pattern
		},
		{
			name:           "command with go file pattern in stderr",
			bin:            "sh",
			args:           []string{"-c", "echo 'stdout message'; echo 'install.go:225: debug message' >&2"},
			wantErr:        false,
			expectedStdout: "stdout message\n",
			expectedStderr: "install.go:225: debug message\n",
			expectedLogs:   []string{"helm: install.go:225: debug message"}, // Go file pattern should be logged with helm prefix
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs []string
			logFn := func(format string, v ...any) {
				logs = append(logs, fmt.Sprintf(format, v...))
			}

			executor := newBinaryExecutor(tt.bin)
			stdout, stderr, err := executor.ExecuteCommand(t.Context(), nil, logFn, tt.args...)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify output is captured in buffers
			assert.Equal(t, tt.expectedStdout, stdout)
			assert.Equal(t, tt.expectedStderr, stderr)

			// Verify logging occurred with expected messages
			assert.ElementsMatch(t, tt.expectedLogs, logs)
		})
	}
}

func Test_logWriter_Write(t *testing.T) {
	var loggedMessages []string
	logFn := func(format string, v ...any) {
		loggedMessages = append(loggedMessages, fmt.Sprintf(format, v...))
	}

	writer := &logWriter{logFn: logFn}

	// Test writing data that matches .go file pattern
	n, err := writer.Write([]byte("install.go:225: test message"))
	assert.NoError(t, err)
	assert.Equal(t, 28, n)
	assert.Len(t, loggedMessages, 1)
	assert.Equal(t, "helm: install.go:225: test message", loggedMessages[0])

	// Test writing data that doesn't match .go file pattern (should be filtered out)
	loggedMessages = nil
	n, err = writer.Write([]byte("verbose debug message"))
	assert.NoError(t, err)
	assert.Equal(t, 21, n)
	assert.Len(t, loggedMessages, 0) // Should be filtered out

	// Test writing empty data
	loggedMessages = nil
	n, err = writer.Write([]byte{})
	assert.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Len(t, loggedMessages, 0)

	// Test with nil logFn
	writer = &logWriter{logFn: nil}
	n, err = writer.Write([]byte("test"))
	assert.NoError(t, err)
	assert.Equal(t, 4, n)
}

func Test_MockBinaryExecutor_ExecuteCommand(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*MockBinaryExecutor)
		env            map[string]string
		args           []string
		expectedStdout string
		expectedStderr string
		expectedErr    error
	}{
		{
			name: "successful command",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything,
					map[string]string{"TEST": "value"},
					mock.Anything, // LogFn
					[]string{"version"},
				).Return("v3.12.0", "", nil)
			},
			env:            map[string]string{"TEST": "value"},
			args:           []string{"version"},
			expectedStdout: "v3.12.0",
			expectedStderr: "",
			expectedErr:    nil,
		},
		{
			name: "command with error",
			setupMock: func(m *MockBinaryExecutor) {
				m.On("ExecuteCommand",
					mock.Anything,
					mock.Anything,
					mock.Anything, // LogFn
					[]string{"invalid"},
				).Return("", "command not found", assert.AnError)
			},
			env:            nil,
			args:           []string{"invalid"},
			expectedStdout: "",
			expectedStderr: "command not found",
			expectedErr:    assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockBinaryExecutor{}
			tt.setupMock(mock)

			stdout, stderr, err := mock.ExecuteCommand(t.Context(), tt.env, nil, tt.args...)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedStderr, stderr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedStdout, stdout)
				assert.Equal(t, tt.expectedStderr, stderr)
			}

			mock.AssertExpectations(t)
		})
	}
}
