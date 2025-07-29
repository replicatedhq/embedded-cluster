package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_HTTPProxy(t *testing.T) {
	// TODO: implement
	engine := NewEngine(nil)
	err := engine.Parse("{{repl HTTPProxy }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestEngine_HTTPSProxy(t *testing.T) {
	// TODO: implement
	engine := NewEngine(nil)
	err := engine.Parse("{{repl HTTPSProxy }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestEngine_NoProxy(t *testing.T) {
	// TODO: implement
	engine := NewEngine(nil)
	err := engine.Parse("{{repl NoProxy }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}
