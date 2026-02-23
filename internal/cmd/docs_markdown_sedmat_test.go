package cmd

import (
	"strings"
	"testing"
)

func TestMarkdownToSedmatExprs_Headings(t *testing.T) {
	md := "# Title\n## Subtitle\n### Section"
	exprs := MarkdownToSedmatExprs(md)
	if len(exprs) != 3 {
		t.Fatalf("expected 3 expressions, got %d: %v", len(exprs), exprs)
	}
	if !strings.Contains(exprs[0], "# Title") {
		t.Errorf("expected heading 1, got %q", exprs[0])
	}
	if !strings.Contains(exprs[1], "## Subtitle") {
		t.Errorf("expected heading 2, got %q", exprs[1])
	}
	if !strings.Contains(exprs[2], "### Section") {
		t.Errorf("expected heading 3, got %q", exprs[2])
	}
}

func TestMarkdownToSedmatExprs_BoldItalic(t *testing.T) {
	md := "**bold text**\n*italic text*"
	exprs := MarkdownToSedmatExprs(md)
	if len(exprs) != 2 {
		t.Fatalf("expected 2 expressions, got %d: %v", len(exprs), exprs)
	}
	if !strings.Contains(exprs[0], "**bold text**") {
		t.Errorf("expected bold, got %q", exprs[0])
	}
	if !strings.Contains(exprs[1], "*italic text*") {
		t.Errorf("expected italic, got %q", exprs[1])
	}
}

func TestMarkdownToSedmatExprs_MixedInline(t *testing.T) {
	md := "Some **bold** and *italic* text"
	exprs := MarkdownToSedmatExprs(md)
	if len(exprs) != 1 {
		t.Fatalf("expected 1 expression, got %d: %v", len(exprs), exprs)
	}
	expr := exprs[0]
	// Should contain brace expressions for inline formatting
	if !strings.Contains(expr, "{b=bold}") {
		t.Errorf("expected {b=bold} in %q", expr)
	}
	if !strings.Contains(expr, "{i=italic}") {
		t.Errorf("expected {i=italic} in %q", expr)
	}
}

func TestMarkdownToSedmatExprs_Code(t *testing.T) {
	md := "`inline code`"
	exprs := MarkdownToSedmatExprs(md)
	if len(exprs) != 1 {
		t.Fatalf("expected 1 expression, got %d", len(exprs))
	}
	if !strings.Contains(exprs[0], "`inline code`") {
		t.Errorf("expected code, got %q", exprs[0])
	}
}

func TestMarkdownToSedmatExprs_Link(t *testing.T) {
	md := "[click here](https://example.com)"
	exprs := MarkdownToSedmatExprs(md)
	if len(exprs) != 1 {
		t.Fatalf("expected 1 expression, got %d", len(exprs))
	}
	if !strings.Contains(exprs[0], "[click here](https:") {
		t.Errorf("expected link, got %q", exprs[0])
	}
}

func TestMarkdownToSedmatExprs_Strike(t *testing.T) {
	md := "~~strikethrough~~"
	exprs := MarkdownToSedmatExprs(md)
	if len(exprs) != 1 {
		t.Fatalf("expected 1 expression, got %d", len(exprs))
	}
	if !strings.Contains(exprs[0], "~~strikethrough~~") {
		t.Errorf("expected strike, got %q", exprs[0])
	}
}

func TestMarkdownToSedmatExprs_Lists(t *testing.T) {
	md := "- item one\n- item two\n1. numbered"
	exprs := MarkdownToSedmatExprs(md)
	if len(exprs) != 3 {
		t.Fatalf("expected 3 expressions, got %d: %v", len(exprs), exprs)
	}
	if !strings.Contains(exprs[0], "- item one") {
		t.Errorf("expected bullet, got %q", exprs[0])
	}
	if !strings.Contains(exprs[2], "1. numbered") {
		t.Errorf("expected numbered, got %q", exprs[2])
	}
}

func TestMarkdownToSedmatExprs_Table(t *testing.T) {
	md := "| A | B |\n|---|---|\n| 1 | 2 |\n| 3 | 4 |"
	exprs := MarkdownToSedmatExprs(md)
	if len(exprs) != 1 {
		t.Fatalf("expected 1 expression for table, got %d: %v", len(exprs), exprs)
	}
	// Should contain pipe-table syntax
	if !strings.Contains(exprs[0], "|") {
		t.Errorf("expected pipe table, got %q", exprs[0])
	}
}

func TestMarkdownToSedmatExprs_HorizontalRule(t *testing.T) {
	md := "---"
	exprs := MarkdownToSedmatExprs(md)
	if len(exprs) != 1 {
		t.Fatalf("expected 1 expression, got %d", len(exprs))
	}
	if !strings.Contains(exprs[0], "---") {
		t.Errorf("expected horizontal rule, got %q", exprs[0])
	}
}

func TestMarkdownToSedmatExprs_CodeBlock(t *testing.T) {
	md := "```go\nfmt.Println(\"hello\")\n```"
	exprs := MarkdownToSedmatExprs(md)
	if len(exprs) != 1 {
		t.Fatalf("expected 1 expression, got %d: %v", len(exprs), exprs)
	}
	if !strings.Contains(exprs[0], "```") {
		t.Errorf("expected code block, got %q", exprs[0])
	}
}

func TestMarkdownToSedmatExprs_EmptyLines(t *testing.T) {
	md := "first\n\nsecond"
	exprs := MarkdownToSedmatExprs(md)
	if len(exprs) != 2 {
		t.Fatalf("expected 2 expressions (empty lines skipped), got %d: %v", len(exprs), exprs)
	}
}

func TestMarkdownToSedmatExprs_Blockquote(t *testing.T) {
	md := "> quoted text"
	exprs := MarkdownToSedmatExprs(md)
	if len(exprs) != 1 {
		t.Fatalf("expected 1 expression, got %d", len(exprs))
	}
	if !strings.Contains(exprs[0], "> quoted text") {
		t.Errorf("expected blockquote, got %q", exprs[0])
	}
}

func TestMarkdownToSedmatExprs_AllExprsPrefixed(t *testing.T) {
	md := "# Heading\nSome text\n- list item\n**bold**"
	exprs := MarkdownToSedmatExprs(md)
	for i, expr := range exprs {
		if !strings.HasPrefix(expr, "s/$/") {
			t.Errorf("expression %d should start with s/$/, got %q", i, expr)
		}
		if !strings.HasSuffix(expr, "/") {
			t.Errorf("expression %d should end with /, got %q", i, expr)
		}
	}
}

func TestMarkdownToSedmatExprs_NestedBoldItalic(t *testing.T) {
	md := "***bold and italic***"
	exprs := MarkdownToSedmatExprs(md)
	if len(exprs) != 1 {
		t.Fatalf("expected 1 expression, got %d", len(exprs))
	}
	if !strings.Contains(exprs[0], "***bold and italic***") {
		t.Errorf("expected bold+italic markdown, got %q", exprs[0])
	}
}

func TestMarkdownToSedmatExprs_MixedInlineWithLink(t *testing.T) {
	md := "Click **here** or [link](https://example.com) now"
	exprs := MarkdownToSedmatExprs(md)
	if len(exprs) != 1 {
		t.Fatalf("expected 1 expression, got %d", len(exprs))
	}
	expr := exprs[0]
	if !strings.Contains(expr, "{b=here}") {
		t.Errorf("expected {b=here}, got %q", expr)
	}
	if !strings.Contains(expr, "{u=") {
		t.Errorf("expected {u=...} for link, got %q", expr)
	}
}

func TestConvertMixedInline(t *testing.T) {
	tests := []struct {
		input    string
		contains []string
	}{
		{"plain text", []string{"plain text"}},
		{"**all bold**", []string{"**all bold**"}}, // single format kept as markdown
		{"Some **bold** word", []string{"Some ", "{b=bold}", " word"}},
		{"a `code` b", []string{"a ", "{#=code}", " b"}},
		{"~~strike~~ it", []string{"{-=strike}", " it"}},
	}
	for _, tt := range tests {
		result := convertInlineToBraces(tt.input)
		for _, want := range tt.contains {
			if !strings.Contains(result, want) {
				t.Errorf("convertInlineToBraces(%q) = %q, want containing %q", tt.input, result, want)
			}
		}
	}
}

func TestMarkdownToSedmatExprs_NestedList(t *testing.T) {
	md := "- top\n  - nested"
	exprs := MarkdownToSedmatExprs(md)
	if len(exprs) != 2 {
		t.Fatalf("expected 2 expressions, got %d: %v", len(exprs), exprs)
	}
	if !strings.Contains(exprs[1], "  - nested") {
		t.Errorf("expected nested list, got %q", exprs[1])
	}
}
