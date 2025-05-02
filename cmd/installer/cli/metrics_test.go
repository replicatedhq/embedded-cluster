package cli

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func TestFlagsToStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *pflag.FlagSet
		expected []string
	}{
		{
			name: "empty flags",
			setup: func() *pflag.FlagSet {
				return pflag.NewFlagSet("test", pflag.ContinueOnError)
			},
			expected: nil,
		},
		{
			name: "unset flags",
			setup: func() *pflag.FlagSet {
				fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
				fs.Bool("bool-flag", false, "")
				fs.String("string-flag", "", "")
				fs.Int("int-flag", 0, "")
				return fs
			},
			expected: nil,
		},
		{
			name: "bool flags",
			setup: func() *pflag.FlagSet {
				fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
				fs.Bool("true-flag", false, "")
				fs.Bool("false-flag", true, "")
				fs.Parse([]string{"--true-flag", "--false-flag=false"})
				return fs
			},
			expected: []string{"--false-flag=false", "--true-flag"},
		},
		{
			name: "string flags",
			setup: func() *pflag.FlagSet {
				fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
				fs.String("name", "", "")
				fs.String("empty", "", "")
				fs.Parse([]string{"--name", "value", "--empty", ""})
				return fs
			},
			expected: []string{"--empty", "", "--name", "value"},
		},
		{
			name: "int flags",
			setup: func() *pflag.FlagSet {
				fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
				fs.Int("count", 0, "")
				fs.Int("zero", 1, "")
				fs.Parse([]string{"--count", "42", "--zero", "0"})
				return fs
			},
			expected: []string{"--count", "42", "--zero", "0"},
		},
		{
			name: "mixed flags",
			setup: func() *pflag.FlagSet {
				fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
				fs.Bool("enable", false, "")
				fs.String("name", "", "")
				fs.Int("count", 0, "")
				fs.Parse([]string{"--enable", "--name", "test", "--count", "5"})
				return fs
			},
			expected: []string{"--count", "5", "--enable", "--name", "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := tt.setup()
			result := flagsToStringSlice(fs)
			assert.Equal(t, tt.expected, result)
		})
	}
}
