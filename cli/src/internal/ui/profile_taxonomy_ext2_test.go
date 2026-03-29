package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubmitTaxonomyPromptEditNameEmptyReturnsError(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{})
	model.taxPromptMode = taxPromptEditName
	model.taxPromptInput.SetValue("   ")

	updated, cmd := model.submitTaxonomyPrompt()
	require.NotNil(t, cmd)
	msg, ok := cmd().(errMsg)
	require.True(t, ok)
	assert.Contains(t, msg.err.Error(), "taxonomy name required")
	assert.Equal(t, taxPromptNone, updated.taxPromptMode)
	assert.Equal(t, "", updated.taxPromptInput.Value())
}

func TestSubmitTaxonomyPromptCreateDescriptionCommandSuccessAndError(t *testing.T) {
	now := time.Now()
	_, okClient := testProfileTaxonomyClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/taxonomy/scopes" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		err := json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":          "scope-1",
				"name":        "public",
				"description": "desc",
				"is_builtin":  false,
				"is_active":   true,
				"metadata":    map[string]any{},
				"created_at":  now,
				"updated_at":  now,
			},
		})
		require.NoError(t, err)
	})

	model := NewProfileModel(okClient, &config.Config{})
	model.taxPromptMode = taxPromptCreateDescription
	model.taxPendingName = "public"
	model.taxPromptInput.SetValue("desc")
	model.taxKind = 0

	updated, cmd := model.submitTaxonomyPrompt()
	require.NotNil(t, cmd)
	assert.Equal(t, taxPromptNone, updated.taxPromptMode)
	assert.True(t, updated.taxLoading)
	msg := cmd()
	_, ok := msg.(taxonomyActionDoneMsg)
	assert.True(t, ok)

	_, failingClient := testProfileTaxonomyClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/taxonomy/scopes" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"create failed"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	model = NewProfileModel(failingClient, &config.Config{})
	model.taxPromptMode = taxPromptCreateDescription
	model.taxPendingName = "private"
	model.taxPromptInput.SetValue("desc")
	model.taxKind = 0

	_, cmd = model.submitTaxonomyPrompt()
	require.NotNil(t, cmd)
	errResult, ok := cmd().(errMsg)
	assert.True(t, ok)
	assert.Contains(t, errResult.err.Error(), "create failed")
}

func TestSubmitTaxonomyPromptEditDescriptionCommandSuccessAndError(t *testing.T) {
	now := time.Now()
	var patchedID string
	var patchedName string
	var patchedDesc string

	_, okClient := testProfileTaxonomyClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/taxonomy/scopes/") || r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		patchedID = strings.TrimPrefix(r.URL.Path, "/api/taxonomy/scopes/")
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		patchedName = body["name"].(string)        //nolint:forcetypeassert
		patchedDesc = body["description"].(string) //nolint:forcetypeassert
		err := json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":          patchedID,
				"name":        patchedName,
				"description": patchedDesc,
				"is_builtin":  false,
				"is_active":   true,
				"metadata":    map[string]any{},
				"created_at":  now,
				"updated_at":  now,
			},
		})
		require.NoError(t, err)
	})

	model := NewProfileModel(okClient, &config.Config{})
	model.taxPromptMode = taxPromptEditDescription
	model.taxPendingName = "renamed"
	model.taxPromptInput.SetValue("updated desc")
	model.taxEditID = "scope-1"
	model.taxKind = 0

	updated, cmd := model.submitTaxonomyPrompt()
	require.NotNil(t, cmd)
	assert.Equal(t, taxPromptNone, updated.taxPromptMode)
	assert.Equal(t, "", updated.taxEditID)
	assert.True(t, updated.taxLoading)
	msg := cmd()
	_, ok := msg.(taxonomyActionDoneMsg)
	assert.True(t, ok)
	assert.Equal(t, "scope-1", patchedID)
	assert.Equal(t, "renamed", patchedName)
	assert.Equal(t, "updated desc", patchedDesc)

	_, failingClient := testProfileTaxonomyClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/taxonomy/scopes/") && r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"update failed"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	model = NewProfileModel(failingClient, &config.Config{})
	model.taxPromptMode = taxPromptEditDescription
	model.taxPendingName = "renamed"
	model.taxPromptInput.SetValue("desc")
	model.taxEditID = "scope-1"
	model.taxKind = 0

	_, cmd = model.submitTaxonomyPrompt()
	require.NotNil(t, cmd)
	errResult, ok := cmd().(errMsg)
	assert.True(t, ok)
	assert.Contains(t, errResult.err.Error(), "update failed")
}

func TestLoadTaxonomyAndArchiveActivateErrorPaths(t *testing.T) {
	_, client := testProfileTaxonomyClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/taxonomy/scopes") && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"list failed"}`))
		case r.URL.Path == "/api/taxonomy/scopes/scope-1/archive" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"archive failed"}`))
		case r.URL.Path == "/api/taxonomy/scopes/scope-1/activate" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"activate failed"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewProfileModel(client, &config.Config{})
	model.taxKind = 0
	_, ok := model.loadTaxonomy().(errMsg)
	assert.True(t, ok)

	model.taxItems = []api.TaxonomyEntry{{ID: "scope-1", Name: "public"}}
	model.taxList.SetRows([]table.Row{{"public"}})
	model.taxList.SetCursor(0)

	updated, cmd := model.taxonomyArchiveSelected()
	require.NotNil(t, cmd)
	assert.True(t, updated.taxLoading)
	msg, ok := cmd().(errMsg)
	assert.True(t, ok)
	assert.Contains(t, msg.err.Error(), "archive failed")

	updated, cmd = model.taxonomyActivateSelected()
	require.NotNil(t, cmd)
	assert.True(t, updated.taxLoading)
	msg, ok = cmd().(errMsg)
	assert.True(t, ok)
	assert.Contains(t, msg.err.Error(), "activate failed")
}
