package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigureOutputModeKeepsAutoForPerCommandDefaults(t *testing.T) {
	t.Setenv(outputModeEnv, "")

	require.NoError(t, configureOutputMode("auto", false, OutputModeTable))
	assert.Equal(t, OutputModeJSON, resolveOutputMode(OutputModeJSON))
	assert.Equal(t, OutputModeTable, resolveOutputMode(OutputModeTable))

	require.NoError(t, configureOutputMode("", true, OutputModeTable))
	assert.Equal(t, OutputModePlain, resolveOutputMode(OutputModeJSON))
}

func TestConfigureOutputModeRejectsInvalidValues(t *testing.T) {
	err := configureOutputMode("yaml", false, OutputModeTable)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --output value")
}

func TestWriteCleanJSONOutputModeContracts(t *testing.T) {
	payload := map[string]any{
		"status": "ok",
		"count":  float64(2),
	}

	t.Setenv(outputModeEnv, string(OutputModeJSON))
	var jsonOut bytes.Buffer
	require.NoError(t, writeCleanJSON(&jsonOut, payload))
	assert.Contains(t, jsonOut.String(), "\n  \"status\": \"ok\"")
	var decodedJSON map[string]any
	require.NoError(t, json.Unmarshal(jsonOut.Bytes(), &decodedJSON))

	t.Setenv(outputModeEnv, string(OutputModePlain))
	var plainOut bytes.Buffer
	require.NoError(t, writeCleanJSON(&plainOut, payload))
	assert.NotContains(t, plainOut.String(), "\n  ")
	var decodedPlain map[string]any
	require.NoError(t, json.Unmarshal(plainOut.Bytes(), &decodedPlain))

	t.Setenv(outputModeEnv, string(OutputModeTable))
	var tableOut bytes.Buffer
	require.NoError(t, writeCleanJSON(&tableOut, payload))
	assert.Contains(t, tableOut.String(), "╭")
	assert.Contains(t, tableOut.String(), "status")
	assert.Contains(t, tableOut.String(), "ok")
}
