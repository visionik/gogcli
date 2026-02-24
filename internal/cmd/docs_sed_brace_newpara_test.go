package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBraceNewParagraph(t *testing.T) {
	// {n} should emit a newline in the cleaned text
	cleaned, spans := findBraceExprs("Hello{n}World")
	assert.Equal(t, "Hello\nWorld", cleaned)
	assert.Empty(t, spans, "no formatting spans for {n}")
}

func TestBraceNewParagraph_WithFormatting(t *testing.T) {
	// {n} combined with inline formatting
	cleaned, spans := findBraceExprs("{b=Bold}{n}Normal")
	assert.Equal(t, "Bold\nNormal", cleaned)
	assert.Len(t, spans, 1, "one span for bold")
	assert.Equal(t, 0, spans[0].Start)
	assert.Equal(t, 4, spans[0].End)
}

func TestBraceNewParagraph_Multiple(t *testing.T) {
	cleaned, spans := findBraceExprs("A{n}B{n}C")
	assert.Equal(t, "A\nB\nC", cleaned)
	assert.Empty(t, spans)
}

func TestBraceNewParagraph_HasBraceFormatting(t *testing.T) {
	assert.True(t, hasBraceFormatting("Hello{n}World"))
}

func TestBraceNewParagraph_InSedExpr(t *testing.T) {
	// Full sed expression with {n}
	expr, err := parseFullExpr("s/^$/Line one{n}Line two/")
	assert.NoError(t, err)
	assert.Equal(t, "Line one\nLine two", expr.replacement)
}
