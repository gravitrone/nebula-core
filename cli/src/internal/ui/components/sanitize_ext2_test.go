package components

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeTextSequenceMatrixAndControlHandling(t *testing.T) {
	assert.Equal(t, "", SanitizeText(""))

	input := "A" +
		"\x1b]8;;https://example.com\x07" + // OSC
		"\x1bPabc\x1b\\" + // DCS
		"\x1b_def\x1b\\" + // APC
		"\x1b^ghi\x1b\\" + // PM
		"\x1bXjkl\x1b\\" + // SOS
		"\u202e" + // bidi control
		"\x00" + // control
		"\n" +
		"\t" +
		"B"

	out := SanitizeText(input)
	assert.Equal(t, "A\n\tB", out)
}

func TestSanitizeOneLineEmptyAndWhitespaceCollapseBranches(t *testing.T) {
	assert.Equal(t, "", SanitizeOneLine(""))
	assert.Equal(t, "", SanitizeOneLine("\u202e\x00"))

	out := SanitizeOneLine("  alpha\n\tbeta  ")
	assert.Equal(t, "alpha  beta", out)
}
