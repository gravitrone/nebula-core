package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatAuditValueAdditionalDefaultAndStructuredBranches(t *testing.T) {
	typedNil := (*int)(nil)
	assert.Equal(t, "None", formatAuditValue(typedNil))

	assert.Equal(t, "None", formatAuditValue([]byte{}))

	asMap := formatAuditValue(map[string]any{
		"profile": map[string]any{
			"owner": "alxx",
		},
	})
	assert.Contains(t, asMap, "profile")
	assert.Contains(t, asMap, "owner")

	asStructuredList := formatAuditValue(`[{"k":"v"},{"k":"x"}]`)
	assert.Contains(t, asStructuredList, "k")
	assert.Contains(t, asStructuredList, "v")
	assert.Contains(t, asStructuredList, "x")
}
