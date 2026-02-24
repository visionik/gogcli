package cmd

import (
	"context"
	"fmt"

	"google.golang.org/api/docs/v1"

	"github.com/steipete/gogcli/internal/ui"
)

func (c *DocsSedCmd) doPositionalInsert(ctx context.Context, docsSvc *docs.Service, u *ui.UI, id string, idx int64, replacement string) error {
	return c.doPositionalInsertWithExpr(ctx, docsSvc, u, id, idx, replacement, false, nil)
}

func (c *DocsSedCmd) doPositionalInsertInner(ctx context.Context, docsSvc *docs.Service, u *ui.UI, id string, idx int64, replacement string, isAppend bool) error {
	return c.doPositionalInsertWithExpr(ctx, docsSvc, u, id, idx, replacement, isAppend, nil)
}

func (c *DocsSedCmd) doPositionalInsertWithExpr(ctx context.Context, docsSvc *docs.Service, u *ui.UI, id string, idx int64, replacement string, isAppend bool, expr *sedExpr) error {
	// Check for image syntax first
	imgSpec := parseImageSyntax(replacement)

	// Check for table creation (explicit |RxC| or pipe-table syntax)
	tableSpec := parseTableCreate(replacement)
	if tableSpec == nil {
		tableSpec = parseTableFromPipes(replacement)
	}

	// Use brace information from the parsed expression if available.
	// The replacement text has already been cleaned by parseFullExpr (braces stripped),
	// so we cannot re-extract braces from it. Instead, use the pre-parsed brace data.
	var braceSpans []*braceSpan
	var braceExprGlobal *braceExpr
	if expr != nil && (expr.brace != nil || len(expr.braceSpans) > 0) {
		braceSpans = expr.braceSpans
		braceExprGlobal = expr.brace
	} else if hasBraceFormatting(replacement) {
		// Fallback: try to extract from replacement (for callers that don't pass expr)
		cleaned, spans := findBraceExprs(replacement)
		if len(spans) > 0 {
			replacement = cleaned
			braceSpans = spans
			if len(spans) == 1 && spans[0].IsGlobal {
				braceExprGlobal = spans[0].Expr
			} else {
				braceExprGlobal = mergeBraceSpans(spans)
			}
		}
	}

	// Parse markdown formatting
	plainText, formats := parseMarkdownReplacement(replacement)

	// For append ($) to non-empty docs, prepend a newline to create a new paragraph.
	// idx > 1 means the document already has content (index 1 = start of body).
	if isAppend && idx > 1 {
		plainText = "\n" + plainText
	}

	var requests []*docs.Request

	switch {
	case tableSpec != nil:
		// For tables, also prepend newline on append to avoid merging with previous content
		if isAppend && idx > 1 {
			requests = append(requests, &docs.Request{
				InsertText: &docs.InsertTextRequest{
					Location: &docs.Location{Index: idx},
					Text:     "\n",
				},
			})
			idx++ // adjust index after newline insertion
		}
		// Insert a table at the position
		requests = append(requests, &docs.Request{
			InsertTable: &docs.InsertTableRequest{
				Location: &docs.Location{Index: idx},
				Rows:     int64(tableSpec.rows),
				Columns:  int64(tableSpec.cols),
			},
		})
	case imgSpec != nil:
		// Insert an image
		imgReq := &docs.InsertInlineImageRequest{
			Uri:        imgSpec.URL,
			Location:   &docs.Location{Index: idx},
			ObjectSize: buildImageSizeSpec(imgSpec),
		}
		requests = append(requests, &docs.Request{InsertInlineImage: imgReq})
	default:
		// Insert plain text
		requests = append(requests, &docs.Request{
			InsertText: &docs.InsertTextRequest{
				Location: &docs.Location{Index: idx},
				Text:     plainText,
			},
		})

		// Apply text formatting if any (from markdown shorthand like **bold**)
		if len(formats) > 0 {
			end := idx + int64(len(plainText))
			requests = append(requests, buildTextStyleRequests(formats, idx, end)...)
		}

		// Brace expression formatting is applied in a separate API call after the insert,
		// because Google Docs may not reliably apply text style changes to text inserted
		// in the same batch request.
	}

	// After inserting text, reset paragraph styles and apply heading/bullet if requested
	if tableSpec == nil && imgSpec == nil && plainText != "" {
		// Calculate range for paragraph styling (skip leading \n)
		paraStart := idx
		if isAppend && idx > 1 {
			paraStart = idx + 1 // style the new paragraph, not the preceding one
		}
		end := idx + int64(len(plainText))

		// Reset paragraph to NORMAL_TEXT (removes inherited heading styles)
		requests = append(requests, &docs.Request{
			UpdateParagraphStyle: &docs.UpdateParagraphStyleRequest{
				Range: &docs.Range{StartIndex: paraStart, EndIndex: end},
				ParagraphStyle: &docs.ParagraphStyle{
					NamedStyleType: "NORMAL_TEXT",
				},
				Fields: "namedStyleType",
			},
		})
		// Remove inherited bullets
		requests = append(requests, &docs.Request{
			DeleteParagraphBullets: &docs.DeleteParagraphBulletsRequest{
				Range: &docs.Range{StartIndex: paraStart, EndIndex: end},
			},
		})

		// Apply heading/bullet formats from markdown
		requests = append(requests, buildParagraphStyleRequests(formats, paraStart, end)...)

		// Apply paragraph-level brace formatting (headings, alignment, etc.)
		if braceExprGlobal != nil && hasBraceParagraphFormat(braceExprGlobal) {
			requests = append(requests, buildBraceParagraphStyleRequests(braceExprGlobal, paraStart, end)...)
		}
	}

	err := retryOnQuota(ctx, func() error {
		_, e := docsSvc.Documents.BatchUpdate(id, &docs.BatchUpdateDocumentRequest{
			Requests: requests,
		}).Context(ctx).Do()
		return e
	})
	if err != nil {
		return fmt.Errorf("batch update (positional insert): %w", err)
	}

	// Apply brace expression formatting in a separate API call.
	// Google Docs does not reliably apply text style changes to text
	// that was inserted in the same batch request.
	if tableSpec == nil && imgSpec == nil && len(braceSpans) > 0 {
		textStart := idx
		if isAppend && idx > 1 {
			textStart = idx + 1 // skip the prepended newline
		}

		var fmtRequests []*docs.Request
		if braceExprGlobal != nil && braceExprHasAnyFormat(braceExprGlobal) {
			textEnd := idx + int64(len(plainText))
			fmtRequests = append(fmtRequests, buildBraceTextStyleRequests(braceExprGlobal, textStart, textEnd)...)
		}
		fmtRequests = append(fmtRequests, buildBraceInlineRequests(braceSpans, textStart)...)

		if len(fmtRequests) > 0 {
			err = retryOnQuota(ctx, func() error {
				_, e := docsSvc.Documents.BatchUpdate(id, &docs.BatchUpdateDocumentRequest{
					Requests: fmtRequests,
				}).Context(ctx).Do()
				return e
			})
			if err != nil {
				return fmt.Errorf("batch update (brace formatting): %w", err)
			}
		}
	}

	// Fill pipe-table cells if content was provided
	if tableSpec != nil && len(tableSpec.cells) > 0 {
		if err := c.fillTableCells(ctx, docsSvc, id, idx, tableSpec); err != nil {
			return fmt.Errorf("fill table cells: %w", err)
		}
	}

	label := fmt.Sprintf("%d chars", len(plainText))
	if tableSpec != nil {
		label = fmt.Sprintf("%dx%d table", tableSpec.rows, tableSpec.cols)
		if len(tableSpec.cells) > 0 {
			label += " (filled)"
		}
	} else if imgSpec != nil {
		label = "image"
	}

	return sedOutputOK(ctx, u, id, sedOutputKV{"inserted", label})
}
