package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleOnboardingKeysRequiresUsername handles test handle onboarding keys requires username.
func TestHandleOnboardingKeysRequiresUsername(t *testing.T) {
	app := NewApp(nil, nil)
	app.onboarding = true

	model, cmd := app.handleOnboardingKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)

	updated := model.(App)
	assert.Equal(t, "username is required", updated.err)
	assert.False(t, updated.onboardingBusy)
}

// TestHandleOnboardingKeysEditsUsernameBuffer handles test handle onboarding keys edits username buffer.
func TestHandleOnboardingKeysEditsUsernameBuffer(t *testing.T) {
	app := NewApp(nil, nil)
	app.onboarding = true
	app.onboardingInput.SetValue("ab")

	model, _ := app.handleOnboardingKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	updated := model.(App)
	assert.Equal(t, "a", updated.onboardingInput.Value())

	model, _ = updated.handleOnboardingKeys(tea.KeyPressMsg{Code: 'z', Text: "z"})
	updated = model.(App)
	assert.Equal(t, "az", updated.onboardingInput.Value())
}

// TestOnboardingLoginCmdReturnsLoginPayload handles test onboarding login cmd returns login payload.
func TestOnboardingLoginCmdReturnsLoginPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/keys/login" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var payload map[string]string
		_ = json.NewDecoder(r.Body).Decode(&payload)
		assert.Equal(t, "alxx", payload["username"])
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"api_key":   "nbl_onboarding",
				"entity_id": "ent-123",
				"username":  "alxx",
			},
		})
	}))
	t.Cleanup(srv.Close)

	app := NewApp(api.NewClient(srv.URL, ""), nil)
	cmd := app.onboardingLoginCmd("  alxx  ")
	require.NotNil(t, cmd)

	msg := cmd()
	done, ok := msg.(onboardingLoginDoneMsg)
	require.True(t, ok)
	require.NoError(t, done.err)
	require.NotNil(t, done.resp)
	assert.Equal(t, "nbl_onboarding", done.resp.APIKey)
	assert.Equal(t, "ent-123", done.resp.EntityID)
	assert.Equal(t, "alxx", done.resp.Username)
}

// TestRenderOnboardingSanitizesAndShowsBusyState handles test render onboarding sanitizes and shows busy state.
func TestRenderOnboardingSanitizesAndShowsBusyState(t *testing.T) {
	app := NewApp(nil, nil)
	app.width = 80
	app.onboarding = true
	app.onboardingInput.SetValue("a\x1b]0;evil\x07\u202E")

	out := app.renderOnboarding()
	clean := stripANSI(out)

	assert.Contains(t, clean, "Username")
	assert.Contains(t, clean, "Press Enter to login.")
	assert.Contains(t, clean, "█")
	assert.NotContains(t, clean, "\u202E")
	assert.False(t, strings.Contains(out, "\x1b]"))

	app.onboardingBusy = true
	out = app.renderOnboarding()
	clean = stripANSI(out)

	assert.Contains(t, clean, "Logging in...")
	assert.NotContains(t, clean, "█")
}
