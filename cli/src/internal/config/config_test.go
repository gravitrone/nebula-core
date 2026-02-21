package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSaveConfigCreatesDirectories handles test save config creates directories.
func TestSaveConfigCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", dir))
	defer func() {
		require.NoError(t, os.Setenv("HOME", oldHome))
	}()

	cfg := Config{
		APIKey: "test-key",
	}

	err := cfg.Save()
	require.NoError(t, err)

	// Verify file exists and has correct permissions
	path := Path()
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

// TestLoadConfigNonExistent handles test load config non existent.
func TestLoadConfigNonExistent(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", dir))
	defer func() {
		require.NoError(t, os.Setenv("HOME", oldHome))
	}()

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestSaveLoadRoundtripWithAllFields handles test save load roundtrip with all fields.
func TestSaveLoadRoundtripWithAllFields(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", dir))
	defer func() {
		require.NoError(t, os.Setenv("HOME", oldHome))
	}()

	original := Config{
		APIKey:            "nbl_verylongkeystring12345",
		UserEntityID:      "ent-123",
		Username:          "testuser",
		Theme:             "dark",
		VimKeys:           true,
		QuickstartPending: true,
	}

	err := original.Save()
	require.NoError(t, err)

	loaded, err := Load()
	require.NoError(t, err)

	assert.Equal(t, original.APIKey, loaded.APIKey)
	assert.Equal(t, original.UserEntityID, loaded.UserEntityID)
	assert.Equal(t, original.Username, loaded.Username)
	assert.Equal(t, original.Theme, loaded.Theme)
	assert.Equal(t, original.VimKeys, loaded.VimKeys)
	assert.Equal(t, original.QuickstartPending, loaded.QuickstartPending)
}

// TestSaveConfigOverwritesExisting handles test save config overwrites existing.
func TestSaveConfigOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", dir))
	defer func() {
		require.NoError(t, os.Setenv("HOME", oldHome))
	}()

	// First save
	cfg1 := Config{APIKey: "key1"}
	err := cfg1.Save()
	require.NoError(t, err)

	// Overwrite
	cfg2 := Config{APIKey: "key2"}
	err = cfg2.Save()
	require.NoError(t, err)

	// Verify second config is loaded
	loaded, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "key2", loaded.APIKey)
}

// TestLoadConfigEmptyFile handles test load config empty file.
func TestLoadConfigEmptyFile(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", dir))
	defer func() {
		require.NoError(t, os.Setenv("HOME", oldHome))
	}()

	// Create .nebula dir and empty config
	cfgDir := filepath.Join(dir, ".nebula")
	require.NoError(t, os.MkdirAll(cfgDir, 0700))
	path := filepath.Join(cfgDir, "config")

	err := os.WriteFile(path, []byte(""), 0600)
	require.NoError(t, err)

	_, err = Load()
	assert.Error(t, err)
}

// TestLoadConfigInvalidYAML handles test load config invalid yaml.
func TestLoadConfigInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", dir))
	defer func() {
		require.NoError(t, os.Setenv("HOME", oldHome))
	}()

	cfgDir := filepath.Join(dir, ".nebula")
	require.NoError(t, os.MkdirAll(cfgDir, 0700))
	path := filepath.Join(cfgDir, "config")

	err := os.WriteFile(path, []byte("invalid: yaml: content:"), 0600)
	require.NoError(t, err)

	_, err = Load()
	assert.Error(t, err)
}

// TestSaveConfigWithEmptyAPIKey handles test save config with empty apikey.
func TestSaveConfigWithEmptyAPIKey(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", dir))
	defer func() {
		require.NoError(t, os.Setenv("HOME", oldHome))
	}()

	cfg := Config{
		APIKey: "", // Empty key should fail on load
	}

	err := cfg.Save()
	require.NoError(t, err)

	_, err = Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_key")
}

// TestConfigPermissionsStrictlyEnforced handles test config permissions strictly enforced.
func TestConfigPermissionsStrictlyEnforced(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", dir))
	defer func() {
		require.NoError(t, os.Setenv("HOME", oldHome))
	}()

	cfg := Config{APIKey: "secret"}
	err := cfg.Save()
	require.NoError(t, err)

	// Try to make it world-readable
	path := Path()
	err = os.Chmod(path, 0644)
	require.NoError(t, err)

	// LoadConfig should fail with incorrect permissions
	_, err = Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permissions")
}

// TestLoadConfigWithLegacyServerURLAndMissingAPIKey handles test load config with legacy server urland missing apikey.
func TestLoadConfigWithLegacyServerURLAndMissingAPIKey(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", dir))
	defer func() {
		require.NoError(t, os.Setenv("HOME", oldHome))
	}()

	cfgDir := filepath.Join(dir, ".nebula")
	require.NoError(t, os.MkdirAll(cfgDir, 0700))
	path := filepath.Join(cfgDir, "config")

	// Legacy server_url is ignored, but missing api_key still fails.
	err := os.WriteFile(path, []byte("server_url: http://legacy\n"), 0600)
	require.NoError(t, err)

	_, err = Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_key")
}

// TestLoadConfigIgnoresLegacyServerURLField handles test load config ignores legacy server urlfield.
func TestLoadConfigIgnoresLegacyServerURLField(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", dir))
	defer func() {
		require.NoError(t, os.Setenv("HOME", oldHome))
	}()

	cfgDir := filepath.Join(dir, ".nebula")
	require.NoError(t, os.MkdirAll(cfgDir, 0700))
	path := filepath.Join(cfgDir, "config")

	err := os.WriteFile(path, []byte("server_url: http://legacy\napi_key: key123\nusername: test\n"), 0600)
	require.NoError(t, err)

	loaded, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "key123", loaded.APIKey)
	assert.Equal(t, "test", loaded.Username)
}

// TestPathReturnsCorrectLocation handles test path returns correct location.
func TestPathReturnsCorrectLocation(t *testing.T) {
	path := Path()
	assert.Contains(t, path, ".nebula")
	assert.Contains(t, path, "config")
}
