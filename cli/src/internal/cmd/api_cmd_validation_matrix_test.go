package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAPICmdParamCommandsRejectInvalidPair(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	commands := [][]string{
		{"entities", "query", "--param", "badpair"},
		{"context", "query", "--param", "badpair"},
		{"relationships", "query", "--param", "badpair"},
		{"jobs", "query", "--param", "badpair"},
		{"logs", "query", "--param", "badpair"},
		{"files", "query", "--param", "badpair"},
		{"protocols", "query", "--param", "badpair"},
		{"audit", "query", "--param", "badpair"},
		{"export", "entities", "--param", "badpair"},
		{"export", "context", "--param", "badpair"},
		{"export", "relationships", "--param", "badpair"},
		{"export", "jobs", "--param", "badpair"},
		{"export", "snapshot", "--param", "badpair"},
	}

	for _, args := range commands {
		runAPISubcommandExpectError(t, "invalid --param value", args...)
	}
}

func TestAPICmdInputCommandsRejectMissingInput(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	commands := [][]string{
		{"entities", "create"},
		{"entities", "update", "e1"},
		{"entities", "bulk-tags"},
		{"entities", "bulk-scopes"},
		{"context", "create"},
		{"context", "update", "c1"},
		{"relationships", "create"},
		{"relationships", "update", "r1"},
		{"jobs", "create"},
		{"jobs", "update", "j1"},
		{"jobs", "subtask", "j1"},
		{"logs", "create"},
		{"logs", "update", "l1"},
		{"files", "create"},
		{"files", "update", "f1"},
		{"protocols", "create"},
		{"protocols", "update", "proto"},
		{"agents", "register"},
		{"agents", "update", "ag1"},
		{"taxonomy", "create", "scopes"},
		{"taxonomy", "update", "scopes", "t1"},
		{"import", "entities"},
		{"import", "context"},
		{"import", "relationships"},
		{"import", "jobs"},
	}

	for _, args := range commands {
		runAPISubcommandExpectError(t, "missing input", args...)
	}
}

func TestAPICmdInputCommandsRejectDecodeTypeMismatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, (&config.Config{APIKey: "nbl_test", Username: "alxx"}).Save())

	commands := [][]string{
		{"entities", "create", "--input", "[]"},
		{"entities", "update", "e1", "--input", "[]"},
		{"entities", "bulk-tags", "--input", "[]"},
		{"entities", "bulk-scopes", "--input", "[]"},
		{"context", "create", "--input", "[]"},
		{"context", "update", "c1", "--input", "[]"},
		{"relationships", "create", "--input", "[]"},
		{"relationships", "update", "r1", "--input", "[]"},
		{"jobs", "create", "--input", "[]"},
		{"jobs", "update", "j1", "--input", "[]"},
		{"jobs", "subtask", "j1", "--input", "[]"},
		{"logs", "create", "--input", "[]"},
		{"logs", "update", "l1", "--input", "[]"},
		{"files", "create", "--input", "[]"},
		{"files", "update", "f1", "--input", "[]"},
		{"protocols", "create", "--input", "[]"},
		{"protocols", "update", "proto", "--input", "[]"},
		{"approvals", "approve", "a1", "--input", "[]"},
		{"agents", "register", "--input", "[]"},
		{"agents", "update", "ag1", "--input", "[]"},
		{"taxonomy", "create", "scopes", "--input", "[]"},
		{"taxonomy", "update", "scopes", "t1", "--input", "[]"},
		{"import", "entities", "--input", "[]"},
		{"import", "context", "--input", "[]"},
		{"import", "relationships", "--input", "[]"},
		{"import", "jobs", "--input", "[]"},
	}

	for _, args := range commands {
		runAPISubcommandExpectError(t, "parse JSON input", args...)
	}
}

func TestAPICmdApprovalsApproveRejectsConflictingInputFlags(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	inputPath := filepath.Join(t.TempDir(), "approve.json")
	require.NoError(t, os.WriteFile(inputPath, []byte(`{"review_notes":"ok"}`), 0o600))

	runAPISubcommandExpectError(
		t,
		"use either --input or --input-file",
		"approvals", "approve", "a1",
		"--input", `{"review_notes":"ok"}`,
		"--input-file", inputPath,
	)
}
