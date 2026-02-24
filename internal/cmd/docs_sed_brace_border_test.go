package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBraceBorder_AllSides(t *testing.T) {
	expr, err := parseBraceExpr("d=1")
	require.NoError(t, err)
	assert.True(t, expr.BorderSet)
	assert.Equal(t, 1.0, expr.BorderAll)
}

func TestBraceBorder_PerSide(t *testing.T) {
	expr, err := parseBraceExpr("dt=2 db=1")
	require.NoError(t, err)
	assert.True(t, expr.BorderSet)
	assert.Equal(t, 2.0, expr.BorderTop)
	assert.Equal(t, 1.0, expr.BorderBottom)
}

func TestBraceBorder_WithColorAndStyle(t *testing.T) {
	expr, err := parseBraceExpr("d=1 dc=red ds=dash")
	require.NoError(t, err)
	assert.True(t, expr.BorderSet)
	assert.Equal(t, 1.0, expr.BorderAll)
	assert.NotEmpty(t, expr.BorderColor) // resolveColor("red")
	assert.Equal(t, "dash", expr.BorderStyle)
}

func TestBraceBorder_Remove(t *testing.T) {
	expr, err := parseBraceExpr("d=0")
	require.NoError(t, err)
	assert.True(t, expr.BorderSet)
	assert.Equal(t, 0.0, expr.BorderAll)
}

func TestBraceBorder_HasParagraphFormat(t *testing.T) {
	expr, _ := parseBraceExpr("d=1")
	assert.True(t, hasBraceParagraphFormat(expr))
}

func TestBraceBorder_HasAnyFormat(t *testing.T) {
	expr, _ := parseBraceExpr("d=1")
	assert.True(t, braceExprHasAnyFormat(expr))
}

func TestBraceBorder_BuildRequests(t *testing.T) {
	expr, _ := parseBraceExpr("d=2 dc=red ds=dash")
	reqs := buildBraceParagraphStyleRequests(expr, 1, 10)
	assert.NotEmpty(t, reqs)
	// Should have an UpdateParagraphStyle with border fields
	req := reqs[0]
	assert.NotNil(t, req.UpdateParagraphStyle)
	ps := req.UpdateParagraphStyle.ParagraphStyle
	assert.NotNil(t, ps.BorderTop)
	assert.NotNil(t, ps.BorderBottom)
	assert.NotNil(t, ps.BorderLeft)
	assert.NotNil(t, ps.BorderRight)
	assert.Equal(t, 2.0, ps.BorderTop.Width.Magnitude)
	assert.Equal(t, "DASH", ps.BorderTop.DashStyle)
}

func TestBraceBorder_PerSideRequests(t *testing.T) {
	expr, _ := parseBraceExpr("dt=1 dc=blue")
	reqs := buildBraceParagraphStyleRequests(expr, 1, 10)
	assert.NotEmpty(t, reqs)
	ps := reqs[0].UpdateParagraphStyle.ParagraphStyle
	assert.NotNil(t, ps.BorderTop)
	assert.Nil(t, ps.BorderBottom)
	assert.Nil(t, ps.BorderLeft)
	assert.Nil(t, ps.BorderRight)
}

func TestBraceBorder_Compound_TopBottom(t *testing.T) {
	expr, err := parseBraceExpr("dtb=2")
	require.NoError(t, err)
	assert.True(t, expr.BorderSet)
	assert.Equal(t, 2.0, expr.BorderTop)
	assert.Equal(t, 2.0, expr.BorderBottom)
	assert.Equal(t, 0.0, expr.BorderLeft)
	assert.Equal(t, 0.0, expr.BorderRight)
}

func TestBraceBorder_Compound_LeftRight(t *testing.T) {
	expr, err := parseBraceExpr("dlr=3")
	require.NoError(t, err)
	assert.True(t, expr.BorderSet)
	assert.Equal(t, 3.0, expr.BorderLeft)
	assert.Equal(t, 3.0, expr.BorderRight)
	assert.Equal(t, 0.0, expr.BorderTop)
	assert.Equal(t, 0.0, expr.BorderBottom)
}

func TestBraceBorder_Compound_AllExplicit(t *testing.T) {
	expr, err := parseBraceExpr("dtblr=1")
	require.NoError(t, err)
	assert.True(t, expr.BorderSet)
	assert.Equal(t, 1.0, expr.BorderTop)
	assert.Equal(t, 1.0, expr.BorderBottom)
	assert.Equal(t, 1.0, expr.BorderLeft)
	assert.Equal(t, 1.0, expr.BorderRight)
}

func TestBraceBorder_Compound_TopRight(t *testing.T) {
	expr, err := parseBraceExpr("dtr=1.5 dc=red")
	require.NoError(t, err)
	assert.True(t, expr.BorderSet)
	assert.Equal(t, 1.5, expr.BorderTop)
	assert.Equal(t, 1.5, expr.BorderRight)
	assert.Equal(t, 0.0, expr.BorderBottom)
	assert.Equal(t, 0.0, expr.BorderLeft)
	assert.NotEmpty(t, expr.BorderColor)
}

func TestBraceBorder_Compound_BuildRequests(t *testing.T) {
	expr, _ := parseBraceExpr("dtb=2 dc=blue")
	reqs := buildBraceParagraphStyleRequests(expr, 1, 10)
	assert.NotEmpty(t, reqs)
	ps := reqs[0].UpdateParagraphStyle.ParagraphStyle
	assert.NotNil(t, ps.BorderTop)
	assert.NotNil(t, ps.BorderBottom)
	assert.Nil(t, ps.BorderLeft)
	assert.Nil(t, ps.BorderRight)
}

func TestBorderDashStyleResolve(t *testing.T) {
	assert.Equal(t, "SOLID", resolveBorderDashStyle(""))
	assert.Equal(t, "SOLID", resolveBorderDashStyle("solid"))
	assert.Equal(t, "DASH", resolveBorderDashStyle("dash"))
	assert.Equal(t, "DASH", resolveBorderDashStyle("dashed"))
	assert.Equal(t, "DOT", resolveBorderDashStyle("dot"))
	assert.Equal(t, "DOT", resolveBorderDashStyle("dotted"))
	assert.Equal(t, "DASH_DOT", resolveBorderDashStyle("dash_dot"))
	assert.Equal(t, "LONG_DASH", resolveBorderDashStyle("long_dash"))
}
