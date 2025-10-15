package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_isAirgap(t *testing.T) {
	tests := []struct {
		name                 string
		isAirgapInstallation bool
		expectedResult       string
	}{
		{
			name:                 "no airgap bundle returns false",
			isAirgapInstallation: false,
			expectedResult:       "false",
		},
		{
			name:                 "airgap bundle set returns true",
			isAirgapInstallation: true,
			expectedResult:       "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(nil, WithMode(ModeGeneric), WithIsAirgap(tt.isAirgapInstallation))
			err := engine.Parse("{{repl IsAirgap }}")
			require.NoError(t, err)

			result, err := engine.Execute(nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestEngine_isAirgapIntegrated(t *testing.T) {
	tests := []struct {
		name                 string
		template             string
		isAirgapInstallation bool
		expectedResult       string
	}{
		{
			name:                 "conditional logic with airgap returns airgap path",
			template:             "{{repl IsAirgap | ternary \"airgap-mode\" \"online-mode\" }}",
			isAirgapInstallation: true,
			expectedResult:       "airgap-mode",
		},
		{
			name:                 "conditional logic without airgap returns online path",
			template:             "{{repl IsAirgap | ternary \"airgap-mode\" \"online-mode\" }}",
			isAirgapInstallation: false,
			expectedResult:       "online-mode",
		},
		{
			name:                 "if statement with airgap",
			template:             "{{repl if IsAirgap }}This is airgap{{repl else }}This is online{{repl end }}",
			isAirgapInstallation: true,
			expectedResult:       "This is airgap",
		},
		{
			name:                 "if statement without airgap",
			template:             "{{repl if IsAirgap }}This is airgap{{repl else }}This is online{{repl end }}",
			isAirgapInstallation: false,
			expectedResult:       "This is online",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(nil, WithMode(ModeGeneric), WithIsAirgap(tt.isAirgapInstallation))
			err := engine.Parse(tt.template)
			require.NoError(t, err)

			result, err := engine.Execute(nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
