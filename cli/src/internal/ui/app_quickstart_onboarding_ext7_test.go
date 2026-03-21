package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleQuickstartKeysNavigationBoundaryMatrix(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.quickstartOpen = true
	app.quickstartStep = 0

	model, cmd := app.handleQuickstartKeys(tea.KeyPressMsg{Code: tea.KeyLeft})
	updated := model.(App)
	require.Nil(t, cmd)
	assert.Equal(t, 0, updated.quickstartStep)

	model, cmd = updated.handleQuickstartKeys(tea.KeyPressMsg{Code: tea.KeyRight})
	updated = model.(App)
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.quickstartStep)

	model, cmd = updated.handleQuickstartKeys(tea.KeyPressMsg{Code: tea.KeyTab})
	updated = model.(App)
	require.Nil(t, cmd)
	assert.Equal(t, 2, updated.quickstartStep)

	model, cmd = updated.handleQuickstartKeys(tea.KeyPressMsg{Code: tea.KeyTab})
	updated = model.(App)
	require.Nil(t, cmd)
	assert.Equal(t, 2, updated.quickstartStep)

	model, cmd = updated.handleQuickstartKeys(tea.KeyPressMsg{Code: 'x', Text: "x"})
	updated = model.(App)
	require.Nil(t, cmd)
	assert.Equal(t, 2, updated.quickstartStep)
}

func TestHandleOnboardingKeysBusyQuitAndEnterBranches(t *testing.T) {
	app := NewApp(nil, nil)
	app.onboarding = true
	app.onboardingName = "alxx"
	app.onboardingBusy = true

	model, cmd := app.handleOnboardingKeys(tea.KeyPressMsg{Code: 'z', Text: "z"})
	updated := model.(App)
	require.Nil(t, cmd)
	assert.Equal(t, "alxx", updated.onboardingName)

	updated.onboardingBusy = false
	model, cmd = updated.handleOnboardingKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	updated = model.(App)
	require.Nil(t, cmd)
	assert.Equal(t, "alx", updated.onboardingName)

	updated.onboardingName = ""
	model, cmd = updated.handleOnboardingKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	updated = model.(App)
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.onboardingName)

	updated.onboardingName = "nebula-user"
	model, cmd = updated.handleOnboardingKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated = model.(App)
	require.NotNil(t, cmd)
	assert.True(t, updated.onboardingBusy)
	assert.Equal(t, "", updated.err)

	model, cmd = updated.handleOnboardingKeys(tea.KeyPressMsg{Code: 'q', Text: "q"})
	updated = model.(App)
	require.NotNil(t, cmd)
	_, ok := cmd().(tea.QuitMsg)
	assert.True(t, ok)
	assert.True(t, updated.onboardingBusy)
}
