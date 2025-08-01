package template

import (
	"encoding/base64"
	"regexp"
	"strings"
	"testing"
	"time"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_Now(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)
	mockInstall := &MockInstallation{proxySpec: &ecv1beta1.ProxySpec{}}

	err := engine.Parse("{{repl Now }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil, WithInstallation(mockInstall))
	require.NoError(t, err)

	// Verify the result is a valid RFC3339 timestamp
	_, err = time.Parse(time.RFC3339, result)
	assert.NoError(t, err, "Result should be a valid RFC3339 timestamp")
}

func TestEngine_NowFmt(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)
	mockInstall := &MockInstallation{proxySpec: &ecv1beta1.ProxySpec{}}

	testCases := []struct {
		name     string
		template string
		validate func(string) bool
	}{
		{
			name:     "RFC3339 format",
			template: "{{repl NowFmt \"2006-01-02T15:04:05Z07:00\" }}",
			validate: func(s string) bool {
				_, err := time.Parse("2006-01-02T15:04:05Z07:00", s)
				return err == nil
			},
		},
		{
			name:     "date only format",
			template: "{{repl NowFmt \"2006-01-02\" }}",
			validate: func(s string) bool {
				_, err := time.Parse("2006-01-02", s)
				return err == nil
			},
		},
		{
			name:     "time only format",
			template: "{{repl NowFmt \"15:04:05\" }}",
			validate: func(s string) bool {
				_, err := time.Parse("15:04:05", s)
				return err == nil
			},
		},
		{
			name:     "empty format defaults to RFC3339",
			template: "{{repl NowFmt \"\" }}",
			validate: func(s string) bool {
				_, err := time.Parse(time.RFC3339, s)
				return err == nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.Parse(tc.template)
			require.NoError(t, err)
			result, err := engine.Execute(nil, WithInstallation(mockInstall))
			require.NoError(t, err)
			assert.True(t, tc.validate(result), "Result should match expected format")
		})
	}
}

func TestEngine_Trim(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)
	mockInstall := &MockInstallation{proxySpec: &ecv1beta1.ProxySpec{}}

	testCases := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "trim whitespace",
			template: "{{repl Trim \"  hello world  \" }}",
			expected: "hello world",
		},
		{
			name:     "trim specific characters",
			template: "{{repl Trim \"***hello***\" \"*\" }}",
			expected: "hello",
		},
		{
			name:     "trim multiple characters",
			template: "{{repl Trim \"###hello###\" \"#\" }}",
			expected: "hello",
		},
		{
			name:     "no trimming needed",
			template: "{{repl Trim \"hello\" }}",
			expected: "hello",
		},
		{
			name:     "empty string",
			template: "{{repl Trim \"\" }}",
			expected: "",
		},
		{
			name:     "only whitespace",
			template: "{{repl Trim \"   \" }}",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.Parse(tc.template)
			require.NoError(t, err)
			result, err := engine.Execute(nil, WithInstallation(mockInstall))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEngine_Base64Encode(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)
	mockInstall := &MockInstallation{proxySpec: &ecv1beta1.ProxySpec{}}

	testCases := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "simple string",
			template: "{{repl Base64Encode \"hello world\" }}",
			expected: "aGVsbG8gd29ybGQ=",
		},
		{
			name:     "empty string",
			template: "{{repl Base64Encode \"\" }}",
			expected: "",
		},
		{
			name:     "special characters",
			template: "{{repl Base64Encode \"hello@world.com\" }}",
			expected: "aGVsbG9Ad29ybGQuY29t",
		},
		{
			name:     "unicode characters",
			template: "{{repl Base64Encode \"hello 世界\" }}",
			expected: "aGVsbG8g5LiW55WM",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.Parse(tc.template)
			require.NoError(t, err)
			result, err := engine.Execute(nil, WithInstallation(mockInstall))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEngine_Base64Decode(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)
	mockInstall := &MockInstallation{proxySpec: &ecv1beta1.ProxySpec{}}

	testCases := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "valid base64",
			template: "{{repl Base64Decode \"aGVsbG8gd29ybGQ=\" }}",
			expected: "hello world",
		},
		{
			name:     "empty string",
			template: "{{repl Base64Decode \"\" }}",
			expected: "",
		},
		{
			name:     "special characters",
			template: "{{repl Base64Decode \"aGVsbG9Ad29ybGQuY29t\" }}",
			expected: "hello@world.com",
		},
		{
			name:     "unicode characters",
			template: "{{repl Base64Decode \"aGVsbG8g5LiW55WM\" }}",
			expected: "hello 世界",
		},
		{
			name:     "invalid base64 returns empty",
			template: "{{repl Base64Decode \"invalid-base64!@#\" }}",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.Parse(tc.template)
			require.NoError(t, err)
			result, err := engine.Execute(nil, WithInstallation(mockInstall))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEngine_RandomString(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)
	mockInstall := &MockInstallation{proxySpec: &ecv1beta1.ProxySpec{}}

	testCases := []struct {
		name     string
		template string
		validate func(string) bool
	}{
		{
			name:     "default charset length 10",
			template: "{{repl RandomString 10 }}",
			validate: func(s string) bool {
				return len(s) == 10 && regexp.MustCompile(`^[_A-Za-z0-9]+$`).MatchString(s)
			},
		},
		{
			name:     "custom charset",
			template: "{{repl RandomString 5 \"[A-Z]\" }}",
			validate: func(s string) bool {
				return len(s) == 5 && regexp.MustCompile(`^[A-Z]+$`).MatchString(s)
			},
		},
		{
			name:     "numeric charset",
			template: "{{repl RandomString 8 \"[0-9]\" }}",
			validate: func(s string) bool {
				return len(s) == 8 && regexp.MustCompile(`^[0-9]+$`).MatchString(s)
			},
		},
		{
			name:     "length 0",
			template: "{{repl RandomString 0 }}",
			validate: func(s string) bool {
				return s == ""
			},
		},
		{
			name:     "invalid charset returns empty",
			template: "{{repl RandomString 5 \"[invalid\" }}",
			validate: func(s string) bool {
				return s == ""
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.Parse(tc.template)
			require.NoError(t, err)
			result, err := engine.Execute(nil, WithInstallation(mockInstall))
			require.NoError(t, err)
			assert.True(t, tc.validate(result), "Result should match expected pattern")
		})
	}
}

func TestEngine_RandomBytes(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)
	mockInstall := &MockInstallation{proxySpec: &ecv1beta1.ProxySpec{}}

	testCases := []struct {
		name     string
		template string
		validate func(string) bool
	}{
		{
			name:     "length 10",
			template: "{{repl RandomBytes 10 }}",
			validate: func(s string) bool {
				decoded, err := base64.StdEncoding.DecodeString(s)
				return err == nil && len(decoded) == 10
			},
		},
		{
			name:     "length 0",
			template: "{{repl RandomBytes 0 }}",
			validate: func(s string) bool {
				decoded, err := base64.StdEncoding.DecodeString(s)
				return err == nil && len(decoded) == 0
			},
		},
		{
			name:     "length 1",
			template: "{{repl RandomBytes 1 }}",
			validate: func(s string) bool {
				decoded, err := base64.StdEncoding.DecodeString(s)
				return err == nil && len(decoded) == 1
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.Parse(tc.template)
			require.NoError(t, err)
			result, err := engine.Execute(nil, WithInstallation(mockInstall))
			require.NoError(t, err)
			assert.True(t, tc.validate(result), "Result should be valid base64 with correct length")
		})
	}
}

func TestEngine_Add(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)
	mockInstall := &MockInstallation{proxySpec: &ecv1beta1.ProxySpec{}}

	testCases := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "int addition",
			template: "{{repl Add 5 3 }}",
			expected: "8",
		},
		{
			name:     "float addition",
			template: "{{repl Add 5.5 3.2 }}",
			expected: "8.7",
		},
		{
			name:     "mixed int and float",
			template: "{{repl Add 5 3.5 }}",
			expected: "8.5",
		},
		{
			name:     "uint addition",
			template: "{{repl Add 10 20 }}",
			expected: "30",
		},
		{
			name:     "negative numbers",
			template: "{{repl Add -5 3 }}",
			expected: "-2",
		},
		{
			name:     "zero addition",
			template: "{{repl Add 0 0 }}",
			expected: "0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.Parse(tc.template)
			require.NoError(t, err)
			result, err := engine.Execute(nil, WithInstallation(mockInstall))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEngine_Sub(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)
	mockInstall := &MockInstallation{proxySpec: &ecv1beta1.ProxySpec{}}

	testCases := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "int subtraction",
			template: "{{repl Sub 10 3 }}",
			expected: "7",
		},
		{
			name:     "float subtraction",
			template: "{{repl Sub 10.5 3.2 }}",
			expected: "7.3",
		},
		{
			name:     "mixed int and float",
			template: "{{repl Sub 10 3.5 }}",
			expected: "6.5",
		},
		{
			name:     "negative result",
			template: "{{repl Sub 3 10 }}",
			expected: "-7",
		},
		{
			name:     "zero subtraction",
			template: "{{repl Sub 5 0 }}",
			expected: "5",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.Parse(tc.template)
			require.NoError(t, err)
			result, err := engine.Execute(nil, WithInstallation(mockInstall))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEngine_Mult(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)
	mockInstall := &MockInstallation{proxySpec: &ecv1beta1.ProxySpec{}}

	testCases := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "int multiplication",
			template: "{{repl Mult 5 3 }}",
			expected: "15",
		},
		{
			name:     "float multiplication",
			template: "{{repl Mult 5.5 3.2 }}",
			expected: "17.6",
		},
		{
			name:     "mixed int and float",
			template: "{{repl Mult 5 3.5 }}",
			expected: "17.5",
		},
		{
			name:     "zero multiplication",
			template: "{{repl Mult 5 0 }}",
			expected: "0",
		},
		{
			name:     "negative multiplication",
			template: "{{repl Mult -5 3 }}",
			expected: "-15",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.Parse(tc.template)
			require.NoError(t, err)
			result, err := engine.Execute(nil, WithInstallation(mockInstall))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEngine_Div(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)
	mockInstall := &MockInstallation{proxySpec: &ecv1beta1.ProxySpec{}}

	testCases := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "int division",
			template: "{{repl Div 10 2 }}",
			expected: "5",
		},
		{
			name:     "float division",
			template: "{{repl Div 10.5 2.5 }}",
			expected: "4.2",
		},
		{
			name:     "mixed int and float",
			template: "{{repl Div 10 2.5 }}",
			expected: "4",
		},

		{
			name:     "negative division",
			template: "{{repl Div -10 2 }}",
			expected: "-5",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.Parse(tc.template)
			require.NoError(t, err)
			result, err := engine.Execute(nil, WithInstallation(mockInstall))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEngine_ParseBool(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)
	mockInstall := &MockInstallation{proxySpec: &ecv1beta1.ProxySpec{}}

	testCases := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "true string",
			template: "{{repl ParseBool \"true\" }}",
			expected: "true",
		},
		{
			name:     "false string",
			template: "{{repl ParseBool \"false\" }}",
			expected: "false",
		},
		{
			name:     "1 string",
			template: "{{repl ParseBool \"1\" }}",
			expected: "true",
		},
		{
			name:     "0 string",
			template: "{{repl ParseBool \"0\" }}",
			expected: "false",
		},
		{
			name:     "invalid string",
			template: "{{repl ParseBool \"invalid\" }}",
			expected: "false",
		},
		{
			name:     "empty string",
			template: "{{repl ParseBool \"\" }}",
			expected: "false",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.Parse(tc.template)
			require.NoError(t, err)
			result, err := engine.Execute(nil, WithInstallation(mockInstall))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEngine_ParseFloat(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)
	mockInstall := &MockInstallation{proxySpec: &ecv1beta1.ProxySpec{}}

	testCases := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "valid float",
			template: "{{repl ParseFloat \"3.14\" }}",
			expected: "3.14",
		},
		{
			name:     "integer string",
			template: "{{repl ParseFloat \"42\" }}",
			expected: "42",
		},
		{
			name:     "negative float",
			template: "{{repl ParseFloat \"-3.14\" }}",
			expected: "-3.14",
		},
		{
			name:     "invalid string",
			template: "{{repl ParseFloat \"invalid\" }}",
			expected: "0",
		},
		{
			name:     "empty string",
			template: "{{repl ParseFloat \"\" }}",
			expected: "0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.Parse(tc.template)
			require.NoError(t, err)
			result, err := engine.Execute(nil, WithInstallation(mockInstall))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEngine_ParseInt(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)
	mockInstall := &MockInstallation{proxySpec: &ecv1beta1.ProxySpec{}}

	testCases := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "decimal string",
			template: "{{repl ParseInt \"42\" }}",
			expected: "42",
		},
		{
			name:     "negative string",
			template: "{{repl ParseInt \"-42\" }}",
			expected: "-42",
		},
		{
			name:     "hex string",
			template: "{{repl ParseInt \"2A\" 16 }}",
			expected: "42",
		},
		{
			name:     "binary string",
			template: "{{repl ParseInt \"101010\" 2 }}",
			expected: "42",
		},
		{
			name:     "invalid string",
			template: "{{repl ParseInt \"invalid\" }}",
			expected: "0",
		},
		{
			name:     "empty string",
			template: "{{repl ParseInt \"\" }}",
			expected: "0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.Parse(tc.template)
			require.NoError(t, err)
			result, err := engine.Execute(nil, WithInstallation(mockInstall))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEngine_ParseUint(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)
	mockInstall := &MockInstallation{proxySpec: &ecv1beta1.ProxySpec{}}

	testCases := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "decimal string",
			template: "{{repl ParseUint \"42\" }}",
			expected: "42",
		},
		{
			name:     "hex string",
			template: "{{repl ParseUint \"2A\" 16 }}",
			expected: "42",
		},
		{
			name:     "binary string",
			template: "{{repl ParseUint \"101010\" 2 }}",
			expected: "42",
		},
		{
			name:     "invalid string",
			template: "{{repl ParseUint \"invalid\" }}",
			expected: "0",
		},
		{
			name:     "empty string",
			template: "{{repl ParseUint \"\" }}",
			expected: "0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.Parse(tc.template)
			require.NoError(t, err)
			result, err := engine.Execute(nil, WithInstallation(mockInstall))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEngine_HumanSize(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)
	mockInstall := &MockInstallation{proxySpec: &ecv1beta1.ProxySpec{}}

	testCases := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "bytes",
			template: "{{repl HumanSize 1024 }}",
			expected: "1.024kB",
		},
		{
			name:     "kilobytes",
			template: "{{repl HumanSize 1048576 }}",
			expected: "1.049MB",
		},
		{
			name:     "megabytes",
			template: "{{repl HumanSize 1073741824 }}",
			expected: "1.074GB",
		},
		{
			name:     "zero",
			template: "{{repl HumanSize 0 }}",
			expected: "0B",
		},
		{
			name:     "float input",
			template: "{{repl HumanSize 1024.5 }}",
			expected: "1.024kB",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.Parse(tc.template)
			require.NoError(t, err)
			result, err := engine.Execute(nil, WithInstallation(mockInstall))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEngine_YamlEscape(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)
	mockInstall := &MockInstallation{proxySpec: &ecv1beta1.ProxySpec{}}

	testCases := []struct {
		name     string
		template string
		validate func(string) bool
	}{
		{
			name:     "simple string",
			template: "{{repl YamlEscape \"hello world\" }}",
			validate: func(s string) bool {
				return strings.Contains(s, "hello world") && strings.HasPrefix(s, "                    ")
			},
		},
		{
			name:     "string with quotes",
			template: "{{repl YamlEscape \"hello \\\"world\\\"\" }}",
			validate: func(s string) bool {
				return strings.Contains(s, "hello") && strings.Contains(s, "world") && strings.HasPrefix(s, "                    ")
			},
		},
		{
			name:     "empty string",
			template: "{{repl YamlEscape \"\" }}",
			validate: func(s string) bool {
				return strings.HasPrefix(s, "                    ")
			},
		},
		{
			name:     "multiline string",
			template: "{{repl YamlEscape \"line1\\nline2\" }}",
			validate: func(s string) bool {
				return strings.Contains(s, "line1") && strings.Contains(s, "line2") && strings.HasPrefix(s, "                    ")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.Parse(tc.template)
			require.NoError(t, err)
			result, err := engine.Execute(nil, WithInstallation(mockInstall))
			require.NoError(t, err)
			assert.True(t, tc.validate(result), "Result should be properly indented YAML")
		})
	}
}
