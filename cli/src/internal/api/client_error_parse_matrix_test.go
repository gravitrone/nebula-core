package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatAPIErrorMatrix(t *testing.T) {
	cases := []struct {
		name    string
		code    string
		message string
		want    string
		ok      bool
	}{
		{name: "code and message", code: "INVALID_API_KEY", message: "key expired", want: "INVALID_API_KEY: key expired", ok: true},
		{name: "code only", code: "FORBIDDEN", message: "", want: "FORBIDDEN", ok: true},
		{name: "message only", code: "", message: "bad request", want: "bad request", ok: true},
		{name: "trimmed values", code: "  E1 ", message: "  spaced ", want: "E1: spaced", ok: true},
		{name: "empty", code: "", message: "", want: "", ok: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := formatAPIError(tc.code, tc.message)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseErrorValueStringMatrix(t *testing.T) {
	got, ok := parseErrorValue("invalid key")
	assert.True(t, ok)
	assert.Equal(t, "invalid key", got)

	got, ok = parseErrorValue("  with spaces  ")
	assert.True(t, ok)
	assert.Equal(t, "with spaces", got)

	got, ok = parseErrorValue("")
	assert.False(t, ok)
	assert.Equal(t, "", got)

	got, ok = parseErrorValue("   ")
	assert.False(t, ok)
	assert.Equal(t, "", got)
}

func TestParseErrorValueMapMatrix(t *testing.T) {
	cases := []struct {
		name string
		raw  any
		want string
		ok   bool
	}{
		{
			name: "code and message",
			raw: map[string]any{
				"code":    "INVALID_API_KEY",
				"message": "key revoked",
			},
			want: "INVALID_API_KEY: key revoked",
			ok:   true,
		},
		{
			name: "nested error object",
			raw: map[string]any{
				"error": map[string]any{
					"code":    "FORBIDDEN",
					"message": "Admin scope required",
				},
			},
			want: "FORBIDDEN: Admin scope required",
			ok:   true,
		},
		{
			name: "message only",
			raw: map[string]any{
				"message": "invalid payload",
			},
			want: "invalid payload",
			ok:   true,
		},
		{
			name: "code only",
			raw: map[string]any{
				"code": "AUTH_REQUIRED",
			},
			want: "AUTH_REQUIRED",
			ok:   true,
		},
		{
			name: "nested empty",
			raw: map[string]any{
				"error": map[string]any{},
			},
			want: "",
			ok:   false,
		},
		{
			name: "map without known fields",
			raw: map[string]any{
				"value": 42,
			},
			want: "",
			ok:   false,
		},
		{
			name: "unsupported type",
			raw:  123,
			want: "",
			ok:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseErrorValue(tc.raw)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestExtractAPIErrorBodyEnvelopeAndPayloadMatrix(t *testing.T) {
	cases := []struct {
		name string
		body map[string]any
		want string
		ok   bool
	}{
		{
			name: "api envelope error",
			body: map[string]any{
				"error": map[string]any{
					"code":    "NOT_FOUND",
					"message": "entity missing",
				},
			},
			want: "NOT_FOUND: entity missing",
			ok:   true,
		},
		{
			name: "detail string",
			body: map[string]any{
				"detail": "invalid token",
			},
			want: "invalid token",
			ok:   true,
		},
		{
			name: "detail nested error",
			body: map[string]any{
				"detail": map[string]any{
					"error": map[string]any{
						"code":    "FORBIDDEN",
						"message": "Admin scope required",
					},
				},
			},
			want: "FORBIDDEN: Admin scope required",
			ok:   true,
		},
		{
			name: "error message only",
			body: map[string]any{
				"error": map[string]any{
					"message": "bad request",
				},
			},
			want: "bad request",
			ok:   true,
		},
		{
			name: "empty envelope error falls back to detail",
			body: map[string]any{
				"error":  map[string]any{},
				"detail": "token expired",
			},
			want: "token expired",
			ok:   true,
		},
		{
			name: "no recognized shape",
			body: map[string]any{
				"status": "error",
			},
			want: "",
			ok:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.body)
			require.NoError(t, err)
			got, ok := extractAPIErrorBody(raw)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestExtractAPIErrorBodyRejectsEmptyAndInvalidJSON(t *testing.T) {
	got, ok := extractAPIErrorBody(nil)
	assert.False(t, ok)
	assert.Equal(t, "", got)

	got, ok = extractAPIErrorBody([]byte("{}"))
	assert.False(t, ok)
	assert.Equal(t, "", got)

	got, ok = extractAPIErrorBody([]byte("{not-json"))
	assert.False(t, ok)
	assert.Equal(t, "", got)
}

func TestNormalizeAPIErrorMatrix(t *testing.T) {
	cases := []struct {
		name   string
		status int
		msg    string
		want   string
	}{
		{
			name:   "401 normalizes to invalid key",
			status: 401,
			msg:    "UNAUTHORIZED: token expired",
			want:   "INVALID_API_KEY: token expired",
		},
		{
			name:   "403 auth detail normalizes to invalid key",
			status: 403,
			msg:    "FORBIDDEN: missing or invalid authorization",
			want:   "INVALID_API_KEY: missing or invalid authorization",
		},
		{
			name:   "403 admin scope stays forbidden",
			status: 403,
			msg:    "FORBIDDEN: Admin scope required",
			want:   "FORBIDDEN: Admin scope required",
		},
		{
			name:   "multi api marker normalizes to conflict code",
			status: 500,
			msg:    "HTTP 500: Address already in use",
			want:   "MULTIPLE_API_INSTANCES_DETECTED: multiple api instances detected",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, normalizeAPIError(tc.status, tc.msg))
		})
	}
}

func TestNormalizeAPIErrorFallbacks(t *testing.T) {
	assert.Equal(t, "", normalizeAPIError(401, ""))
	assert.Equal(t, "", normalizeAPIError(401, "   "))
	assert.Equal(t, "raw-error", normalizeAPIError(500, "raw-error"))
	assert.Equal(t, "INVALID_API_KEY: invalid api key", normalizeAPIError(401, "FORBIDDEN"))
	assert.Equal(t, "INVALID_API_KEY: invalid api key", normalizeAPIError(500, "INVALID_API_KEY"))
	assert.Equal(
		t,
		"INVALID_API_KEY: token expired",
		normalizeAPIError(500, "invalid_api_key: token expired"),
	)
	assert.Equal(
		t,
		"INVALID_API_KEY: token expired",
		normalizeAPIError(500, "auth_required: token expired"),
	)
}

func TestParseErrorCodeAndAuthDetailMatrix(t *testing.T) {
	code, msg := parseErrorCode("INVALID_API_KEY: token expired")
	assert.Equal(t, "INVALID_API_KEY", code)
	assert.Equal(t, "token expired", msg)

	code, msg = parseErrorCode("HTTP 500: server exploded")
	assert.Equal(t, "", code)
	assert.Equal(t, "HTTP 500: server exploded", msg)

	code, msg = parseErrorCode("bad-code: nope")
	assert.Equal(t, "", code)
	assert.Equal(t, "bad-code: nope", msg)

	code, msg = parseErrorCode("lowercase: nope")
	assert.Equal(t, "", code)
	assert.Equal(t, "lowercase: nope", msg)

	assert.Equal(t, "invalid api key", normalizedAuthDetail("HTTP 401: Unauthorized"))
	assert.Equal(t, "invalid api key", normalizedAuthDetail("FORBIDDEN"))
	assert.Equal(t, "token revoked", normalizedAuthDetail("INVALID_API_KEY: token revoked"))
	assert.Equal(t, "token expired", normalizedAuthDetail("invalid_api_key: token expired"))
	assert.Equal(t, "token expired", normalizedAuthDetail("auth_required: token expired"))
}

func TestShouldNormalizeInvalidAPIKeyMatrix(t *testing.T) {
	assert.True(t, shouldNormalizeInvalidAPIKey(401, "anything"))
	assert.True(t, shouldNormalizeInvalidAPIKey(403, "missing or invalid authorization"))
	assert.True(t, shouldNormalizeInvalidAPIKey(403, "invalid api key"))
	assert.False(t, shouldNormalizeInvalidAPIKey(403, "admin scope required"))
	assert.False(t, shouldNormalizeInvalidAPIKey(500, "internal server error"))
}

func TestDecodeListErrorPath(t *testing.T) {
	_, err := decodeList[map[string]any]([]byte("{bad-json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")
}
