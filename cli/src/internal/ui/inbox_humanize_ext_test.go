package ui

import "testing"

import "github.com/stretchr/testify/assert"

func TestHumanizeApprovalTypeAdditionalBranches(t *testing.T) {
	assert.Equal(t, "", humanizeApprovalType(" \x1b[31m \x1b[0m "))
	assert.Equal(t, "  Bulk  Update  ", humanizeApprovalType("__BULK__UPDATE__"))
	assert.Equal(t, "Create Entity", humanizeApprovalType("create_entity"))
}
