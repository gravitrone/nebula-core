package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type errWriter struct{}

func (errWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestLoadCommandClientOptionalAuthAllowsMissingConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	client, err := loadCommandClient(false)
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestLoadCommandClientRequiredAuthRejectsMissingConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	client, err := loadCommandClient(true)
	require.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestWriteCleanJSONNilFallbackAndEncodeError(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, writeCleanJSON(&out, nil))
	assert.JSONEq(t, `{"ok":true}`, out.String())

	err := writeCleanJSON(errWriter{}, map[string]any{"ok": true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write failed")
}

func TestParseQueryParamsTrimAndEmptyValue(t *testing.T) {
	params, err := parseQueryParams([]string{" limit = 10 ", "q= nebula ", "empty="})
	require.NoError(t, err)
	assert.Equal(t, "10", params["limit"])
	assert.Equal(t, "nebula", params["q"])
	assert.Equal(t, "", params["empty"])
}

func TestReadInputJSONOptionalAndInputFileErrors(t *testing.T) {
	raw, err := readInputJSON("", "", false)
	require.NoError(t, err)
	assert.Nil(t, raw)

	_, err = readInputJSON("", "does-not-exist.json", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read input file")

	badPath := t.TempDir() + "/bad.json"
	require.NoError(t, os.WriteFile(badPath, []byte("{broken"), 0o600))
	_, err = readInputJSON("", badPath, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON in --input-file")
}

func TestDecodeJSONInputBranches(t *testing.T) {
	var payload map[string]any
	err := decodeJSONInput(nil, &payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing JSON input")

	err = decodeJSONInput(json.RawMessage(`{"ok":true}`), &payload)
	require.NoError(t, err)
	assert.Equal(t, true, payload["ok"])

	err = decodeJSONInput(json.RawMessage(`{broken`), &payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse JSON input")
}
