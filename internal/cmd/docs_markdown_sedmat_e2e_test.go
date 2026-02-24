package cmd

import (
	"strings"
	"testing"

	docs "google.golang.org/api/docs/v1"
)

// TestMarkdownSedmatE2E_BoldText verifies that markdown bold text produces
// the correct Google Docs API requests when run through the full pipeline:
// markdown → sedmat expressions → sedmat engine → API requests.
func TestMarkdownSedmatE2E_BoldText(t *testing.T) {
	doc := makeEmptyDoc()
	exprs := MarkdownToSedmatExprs("**Hello World**")
	if len(exprs) == 0 {
		t.Fatal("expected at least one expression")
	}
	reqs := runSedIntegration(t, doc, exprs[0], exprs[1:])
	if len(reqs) == 0 {
		t.Fatal("expected API requests")
	}
	// Should have at least an InsertText and an UpdateTextStyle for bold
	hasInsert := false
	hasBold := false
	for _, r := range reqs {
		if r.InsertText != nil && strings.Contains(r.InsertText.Text, "Hello World") {
			hasInsert = true
		}
		if r.UpdateTextStyle != nil && r.UpdateTextStyle.TextStyle != nil && r.UpdateTextStyle.TextStyle.Bold {
			hasBold = true
		}
	}
	if !hasInsert {
		t.Error("expected InsertText request with 'Hello World'")
	}
	if !hasBold {
		t.Error("expected UpdateTextStyle request with bold=true")
	}
}

// TestMarkdownSedmatE2E_Heading verifies heading markdown produces paragraph style updates.
func TestMarkdownSedmatE2E_Heading(t *testing.T) {
	doc := makeEmptyDoc()
	exprs := MarkdownToSedmatExprs("# My Title")
	if len(exprs) == 0 {
		t.Fatal("expected at least one expression")
	}
	reqs := runSedIntegration(t, doc, exprs[0], exprs[1:])
	if len(reqs) == 0 {
		t.Fatal("expected API requests")
	}
	hasInsert := false
	hasHeading := false
	for _, r := range reqs {
		if r.InsertText != nil && strings.Contains(r.InsertText.Text, "My Title") {
			hasInsert = true
		}
		if r.UpdateParagraphStyle != nil && r.UpdateParagraphStyle.ParagraphStyle != nil {
			style := r.UpdateParagraphStyle.ParagraphStyle.NamedStyleType
			if strings.Contains(style, "HEADING") {
				hasHeading = true
			}
		}
	}
	if !hasInsert {
		t.Error("expected InsertText with 'My Title'")
	}
	if !hasHeading {
		t.Error("expected UpdateParagraphStyle with HEADING style")
	}
}

// TestMarkdownSedmatE2E_MixedContent verifies a realistic markdown doc works end-to-end.
func TestMarkdownSedmatE2E_MixedContent(t *testing.T) {
	md := `# Report

This is a **bold** statement.

- Item one
- Item two

| Name | Value |
|------|-------|
| A    | 1     |
`
	doc := makeEmptyDoc()
	exprs := MarkdownToSedmatExprs(md)
	if len(exprs) < 3 {
		t.Fatalf("expected multiple expressions, got %d", len(exprs))
	}
	reqs := runSedIntegration(t, doc, exprs[0], exprs[1:])
	if len(reqs) == 0 {
		t.Fatal("expected API requests")
	}
	t.Logf("Generated %d expressions, %d API requests", len(exprs), len(reqs))
}

// TestMarkdownSedmatE2E_PlainText verifies plain text without formatting.
func TestMarkdownSedmatE2E_PlainText(t *testing.T) {
	doc := makeEmptyDoc()
	exprs := MarkdownToSedmatExprs("Just some plain text.")
	if len(exprs) == 0 {
		t.Fatal("expected at least one expression")
	}
	reqs := runSedIntegration(t, doc, exprs[0], exprs[1:])
	hasInsert := false
	for _, r := range reqs {
		if r.InsertText != nil && strings.Contains(r.InsertText.Text, "Just some plain text") {
			hasInsert = true
		}
	}
	if !hasInsert {
		t.Error("expected InsertText with plain text")
	}
}

// makeEmptyDoc creates a minimal empty Google Doc for testing.
func makeEmptyDoc() *docs.Document {
	return &docs.Document{
		DocumentId: "test-doc-id",
		Title:      "Test Doc",
		Body: &docs.Body{
			Content: []*docs.StructuralElement{
				{
					StartIndex: 0,
					EndIndex:   1,
					Paragraph: &docs.Paragraph{
						Elements: []*docs.ParagraphElement{
							{
								StartIndex: 0,
								EndIndex:   1,
								TextRun: &docs.TextRun{
									Content: "\n",
								},
							},
						},
					},
				},
			},
		},
	}
}
