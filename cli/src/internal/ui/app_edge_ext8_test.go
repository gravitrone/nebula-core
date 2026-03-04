package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusHintsForTabUnknownTabReturnsBaseHints(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tab = 999

	hints := app.statusHintsForTab()
	require.NotEmpty(t, hints)
	joined := strings.Join(hints, " ")
	assert.Contains(t, joined, "Tabs")
	assert.Contains(t, joined, "Command")
	assert.Contains(t, joined, "Help")
}

func TestRenderToastNilAndDefaultLevelBranches(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.width = 80

	assert.Equal(t, "", app.renderToast())

	app.toast = &appToast{level: "custom", text: "hello"}
	out := app.renderToast()
	assert.Contains(t, out, "hello")
}

func TestRenderQuickstartClampsStepBounds(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.width = 80

	app.quickstartStep = -3
	out := app.renderQuickstart()
	assert.Contains(t, out, "Step 1/3")

	app.quickstartStep = 99
	out = app.renderQuickstart()
	assert.Contains(t, out, "Step 3/3")
}

func TestParseErrorCodeAndMessageAdditionalBranches(t *testing.T) {
	code, msg := parseErrorCodeAndMessage("")
	assert.Equal(t, "", code)
	assert.Equal(t, "", msg)

	code, msg = parseErrorCodeAndMessage("plain failure")
	assert.Equal(t, "", code)
	assert.Equal(t, "plain failure", msg)

	code, msg = parseErrorCodeAndMessage("BAD-CODE: nope")
	assert.Equal(t, "", code)
	assert.Equal(t, "BAD-CODE: nope", msg)
}

func TestShouldShowRecoveryHintsAdditionalBranches(t *testing.T) {
	assert.True(t, shouldShowRecoveryHints("", "HTTP 401 unauthorized"))
	assert.False(t, shouldShowRecoveryHints("", "some unrelated error"))
}

func TestOnboardingLoginCmdNilClientBranch(t *testing.T) {
	app := NewApp(nil, nil)
	cmd := app.onboardingLoginCmd("alxx")
	require.NotNil(t, cmd)

	msg, ok := cmd().(onboardingLoginDoneMsg)
	require.True(t, ok)
	assert.True(t, msg.err != nil || msg.resp != nil)
}

func TestClassifyStartupAuthDefaultFailedBranch(t *testing.T) {
	cfg := &config.Config{APIKey: "nbl_key"}
	assert.Equal(t, "failed", classifyStartupAuth("some random error", cfg))
}

func TestHandleQuickstartKeysLeftDecrementBranch(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.quickstartStep = 2

	model, cmd := app.handleQuickstartKeys(tea.KeyMsg{Type: tea.KeyLeft})
	updated := model.(App)
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.quickstartStep)
}
