package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testProfileTaxonomyClient handles test profile taxonomy client.
func testProfileTaxonomyClient(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *api.Client) {
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, api.NewClient(srv.URL, "test-key")
}

// TestProfileTaxonomyCreateFlowQueuesReload handles test profile taxonomy create flow queues reload.
func TestProfileTaxonomyCreateFlowQueuesReload(t *testing.T) {
	now := time.Now()
	created := false
	listed := false

	_, client := testProfileTaxonomyClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/taxonomy/scopes" && r.Method == http.MethodPost:
			created = true
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":          "scope-new",
					"name":        "team-scope",
					"description": "desc",
					"is_builtin":  false,
					"is_active":   true,
					"metadata":    map[string]any{},
					"created_at":  now,
					"updated_at":  now,
				},
			})
			require.NoError(t, err)
			return
		case strings.HasPrefix(r.URL.Path, "/api/taxonomy/scopes") && r.Method == http.MethodGet:
			listed = true
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":          "scope-new",
						"name":        "team-scope",
						"description": "desc",
						"is_builtin":  false,
						"is_active":   true,
						"metadata":    map[string]any{},
						"created_at":  now,
						"updated_at":  now,
					},
				},
			})
			require.NoError(t, err)
			return
		default:
			// ProfileModel uses other endpoints on Init, but this test drives prompt flow only.
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewProfileModel(client, &config.Config{APIKey: "test-key"})
	model.section = 2

	// Open create prompt.
	model, _ = model.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	assert.Equal(t, taxPromptCreateName, model.taxPromptMode)

	// Type name "team-scope" then submit.
	for _, ch := range "team-scope" {
		model, _ = model.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
	}
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, taxPromptCreateDescription, model.taxPromptMode)

	// Type description then submit, which triggers API call.
	for _, ch := range "desc" {
		model, _ = model.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
	}
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	model, cmd = model.Update(msg)
	require.NotNil(t, cmd)
	msg = cmd()
	model, _ = model.Update(msg)

	assert.True(t, created)
	assert.True(t, listed)
	assert.Equal(t, taxPromptNone, model.taxPromptMode)
	assert.False(t, model.taxLoading)
	assert.Len(t, model.taxItems, 1)
}

// TestProfileTaxonomyArchiveAndActivateFlowQueuesReload handles test profile taxonomy archive and activate flow queues reload.
func TestProfileTaxonomyArchiveAndActivateFlowQueuesReload(t *testing.T) {
	now := time.Now()
	archived := false
	activated := false
	listCalls := 0

	_, client := testProfileTaxonomyClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/taxonomy/scopes/scope-1/archive" && r.Method == http.MethodPost:
			archived = true
			err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "scope-1"}})
			require.NoError(t, err)
			return
		case r.URL.Path == "/api/taxonomy/scopes/scope-1/activate" && r.Method == http.MethodPost:
			activated = true
			err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "scope-1"}})
			require.NoError(t, err)
			return
		case strings.HasPrefix(r.URL.Path, "/api/taxonomy/scopes") && r.Method == http.MethodGet:
			listCalls++
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":          "scope-1",
						"name":        "public",
						"description": "demo",
						"is_builtin":  true,
						"is_active":   true,
						"metadata":    map[string]any{},
						"created_at":  now,
						"updated_at":  now,
					},
				},
			})
			require.NoError(t, err)
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewProfileModel(client, &config.Config{APIKey: "test-key", Username: "alxx"})
	model.section = 2
	model.width = 100
	model.taxKind = 0 // scopes
	model.taxLoading = false
	model.taxItems = []api.TaxonomyEntry{{
		ID:          "scope-1",
		Name:        "public",
		Description: ptrString("demo"),
		IsBuiltin:   true,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}}
	model.taxList.SetItems([]string{formatTaxonomyLine(model.taxItems[0])})

	// Archive selected.
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	require.NotNil(t, cmd)
	model, cmd = model.Update(cmd())
	require.NotNil(t, cmd)
	model, _ = model.Update(cmd())

	assert.True(t, archived)
	assert.GreaterOrEqual(t, listCalls, 1)

	// Activate selected.
	model, cmd = model.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	require.NotNil(t, cmd)
	model, cmd = model.Update(cmd())
	require.NotNil(t, cmd)
	model, _ = model.Update(cmd())

	assert.True(t, activated)
	assert.GreaterOrEqual(t, listCalls, 2)
}

// TestProfileTaxonomyRenderAndHelperCoverage handles test profile taxonomy render and helper coverage.
func TestProfileTaxonomyRenderAndHelperCoverage(t *testing.T) {
	now := time.Now()
	model := NewProfileModel(nil, &config.Config{Username: "alxx", APIKey: "nbl_x"})
	model.section = 2
	model.width = 100
	model.taxKind = 0
	model.taxLoading = false
	model.taxIncludeInactive = true
	model.taxSearch = "pub"
	model.taxItems = []api.TaxonomyEntry{{
		ID:          "scope-1",
		Name:        "public",
		Description: ptrString("demo"),
		IsBuiltin:   true,
		IsActive:    false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}}
	model.taxList.SetItems([]string{formatTaxonomyLine(model.taxItems[0])})

	item := model.selectedTaxonomy()
	require.NotNil(t, item)
	assert.Equal(t, "scope-1", item.ID)

	out := model.renderTaxonomy()
	assert.Contains(t, out, "Scopes")
	assert.Contains(t, out, "rows")
	assert.Contains(t, out, "include inactive: true")

	model.taxPromptMode = taxPromptFilter
	assert.Equal(t, "Taxonomy Filter", model.taxonomyPromptTitle())
}

// ptrString handles ptr string.
func ptrString(s string) *string {
	return &s
}
