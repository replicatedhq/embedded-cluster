package metrics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactFlags(t *testing.T) {
	for _, test := range []struct {
		name     string
		flags    []string
		expected []string
	}{
		{
			name:     "boolean secret flag",
			flags:    []string{"some-command", "--boolean-flag-password"},
			expected: []string{"some-command", "--boolean-flag-password"},
		},
		{
			name:     "boolean flag followed by a flag to redact",
			flags:    []string{"some-command", "--boolean-flag-password", "--admin-console-password", "value"},
			expected: []string{"some-command", "--boolean-flag-password", "--admin-console-password", "*****"},
		},
		{
			name:     "flag to redact followed by other flags",
			flags:    []string{"some-secret-command", "--admin-console-password", "value", "--some-key", "some-value"},
			expected: []string{"some-secret-command", "--admin-console-password", "*****", "--some-key", "some-value"},
		},
		{
			name:     "redacts admin-console-password, http-proxy and https-proxy flags",
			flags:    []string{"some-secret-command", "--admin-console-password", "value", "--https-proxy", "some-value", "--http-proxy", "another-value"},
			expected: []string{"some-secret-command", "--admin-console-password", "*****", "--https-proxy", "*****", "--http-proxy", "*****"},
		},
		{
			name:     "redacts flags using `=` assignement",
			flags:    []string{"some-secret-command", "--admin-console-password=value", "--https-proxy", "some-value", "--no-secret", "another-value"},
			expected: []string{"some-secret-command", "--admin-console-password=*****", "--https-proxy", "*****", "--no-secret", "another-value"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)
			result := redactFlags(test.flags)
			req.Equal(test.expected, result)
		})
	}
}
