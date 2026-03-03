package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveConfigReturnsCreateDirErrorWhenHomeIsFile(t *testing.T) {
	base := t.TempDir()
	homeFile := filepath.Join(base, "home-as-file")
	require.NoError(t, os.WriteFile(homeFile, []byte("x"), 0o600))
	t.Setenv("HOME", homeFile)

	cfg := &Config{APIKey: "nbl_test"}
	err := cfg.Save()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create config dir")
}

func TestLoadConfigReturnsReadErrorWhenConfigPathIsDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfgDir := filepath.Join(home, ".nebula")
	require.NoError(t, os.MkdirAll(cfgDir, 0o700))
	configPath := filepath.Join(cfgDir, "config")
	require.NoError(t, os.Mkdir(configPath, 0o600))

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read config")
}

func TestSaveConfigSetsDefaultPendingLimitWhenMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := &Config{
		APIKey:       "nbl_test",
		PendingLimit: 0,
	}
	require.NoError(t, cfg.Save())
	assert.Equal(t, 500, cfg.PendingLimit)

	loaded, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 500, loaded.PendingLimit)
}

func TestSaveConfigReturnsWriteErrorWhenConfigPathIsDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfgDir := filepath.Join(home, ".nebula")
	require.NoError(t, os.MkdirAll(cfgDir, 0o700))
	require.NoError(t, os.Mkdir(filepath.Join(cfgDir, "config"), 0o700))

	cfg := &Config{APIKey: "nbl_test"}
	err := cfg.Save()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is a directory")
}
