// Package cmd provides CLI commands for Google Docs operations.
package cmd

import (
	"strconv"
	"strings"

	"google.golang.org/api/docs/v1"
)

// buildBraceTextStyleRequests converts a braceExpr to UpdateTextStyle requests.
// It handles boolean flags (bold, italic, etc.), value flags (color, font, size),
// negation, and reset.
func buildBraceTextStyleRequests(be *braceExpr, start, end int64) []*docs.Request {
	if be == nil {
		return nil
	}

	// Handle reset: explicit {0} OR implicit (default unless {!0}).
	// Every brace expression implicitly resets all formatting first,
	// making output deterministic regardless of inherited doc styles.
	// Use {!0} to opt into additive mode (preserve existing styles).
	if be.Reset || !be.NoReset {
		return buildResetTextStyleRequests(be, start, end)
	}

	style := &docs.TextStyle{}
	var fields []string

	// Boolean flags
	if be.Bold != nil {
		style.Bold = *be.Bold
		fields = append(fields, "bold")
	}
	if be.Italic != nil {
		style.Italic = *be.Italic
		fields = append(fields, "italic")
	}
	if be.Underline != nil {
		style.Underline = *be.Underline
		fields = append(fields, "underline")
	}
	if be.Strike != nil {
		style.Strikethrough = *be.Strike
		fields = append(fields, "strikethrough")
	}
	if be.SmallCaps != nil {
		style.SmallCaps = *be.SmallCaps
		fields = append(fields, "smallCaps")
	}
	if be.Sup != nil && *be.Sup {
		style.BaselineOffset = "SUPERSCRIPT"
		fields = append(fields, "baselineOffset")
	}
	if be.Sub != nil && *be.Sub {
		style.BaselineOffset = "SUBSCRIPT"
		fields = append(fields, "baselineOffset")
	}
	// Reset baseline if both sup and sub are explicitly false
	if be.Sup != nil && !*be.Sup && be.Sub != nil && !*be.Sub {
		style.BaselineOffset = "NONE"
		fields = append(fields, "baselineOffset")
	}

	// Code flag: monospace font + grey background
	if be.Code != nil && *be.Code {
		style.WeightedFontFamily = &docs.WeightedFontFamily{FontFamily: "Courier New"}
		style.BackgroundColor = greyColor(codeBackgroundGrey)
		fields = append(fields, "weightedFontFamily", "backgroundColor")
	}

	// Value flags
	if be.Font != "" {
		style.WeightedFontFamily = &docs.WeightedFontFamily{FontFamily: be.Font}
		fields = append(fields, "weightedFontFamily")
	}
	if be.Size > 0 {
		style.FontSize = &docs.Dimension{Magnitude: be.Size, Unit: "PT"}
		fields = append(fields, "fontSize")
	}
	if be.Color != "" {
		if r, g, b, ok := parseHexColor(be.Color); ok {
			style.ForegroundColor = &docs.OptionalColor{
				Color: &docs.Color{RgbColor: &docs.RgbColor{Red: r, Green: g, Blue: b}},
			}
			fields = append(fields, "foregroundColor")
		}
	}
	if be.Bg != "" {
		if r, g, b, ok := parseHexColor(be.Bg); ok {
			style.BackgroundColor = &docs.OptionalColor{
				Color: &docs.Color{RgbColor: &docs.RgbColor{Red: r, Green: g, Blue: b}},
			}
			fields = append(fields, "backgroundColor")
		}
	}
	if be.URL != "" {
		// Handle bookmark links (#name) vs regular URLs
		if strings.HasPrefix(be.URL, "#") {
			style.Link = &docs.Link{BookmarkId: be.URL[1:]}
		} else {
			style.Link = &docs.Link{Url: be.URL}
		}
		fields = append(fields, "link")
	}

	if len(fields) == 0 {
		return nil
	}

	return []*docs.Request{
		{
			UpdateTextStyle: &docs.UpdateTextStyleRequest{
				Range:     &docs.Range{StartIndex: start, EndIndex: end},
				TextStyle: style,
				Fields:    strings.Join(fields, ","),
			},
		},
	}
}

// resetFieldsStr is the pre-joined field mask for resetting all text formatting.
// Package-level to avoid per-call allocation and string joining.
var resetFieldsStr = strings.Join([]string{
	"bold", "italic", "underline", "strikethrough", "smallCaps",
	"baselineOffset", "foregroundColor", "backgroundColor",
	"fontSize", "weightedFontFamily", "link",
}, ",")

// buildResetTextStyleRequests builds requests to clear all formatting, then apply any
// additional flags specified after the reset (e.g., {0 b} = reset then bold).
func buildResetTextStyleRequests(be *braceExpr, start, end int64) []*docs.Request {
	requests := []*docs.Request{
		{
			UpdateTextStyle: &docs.UpdateTextStyleRequest{
				Range:     &docs.Range{StartIndex: start, EndIndex: end},
				TextStyle: &docs.TextStyle{}, // Empty style = reset
				Fields:    resetFieldsStr,
			},
		},
	}

	// Now apply any flags that were set alongside the reset
	// Create a copy without Reset flag and with NoReset to avoid recursion
	postReset := *be
	postReset.Reset = false
	postReset.NoReset = true

	if braceExprHasTextFormat(&postReset) {
		requests = append(requests, buildBraceTextStyleRequests(&postReset, start, end)...)
	}

	return requests
}

// braceExprHasTextFormat returns true if the braceExpr has any text-level formatting.
func braceExprHasTextFormat(be *braceExpr) bool {
	if be == nil {
		return false
	}
	return be.Bold != nil || be.Italic != nil || be.Underline != nil ||
		be.Strike != nil || be.Code != nil || be.Sup != nil ||
		be.Sub != nil || be.SmallCaps != nil || be.Color != "" ||
		be.Bg != "" || be.Font != "" || be.Size > 0 || be.URL != ""
}

// buildBraceParagraphStyleRequests converts a braceExpr to paragraph-level requests.
// Handles headings, alignment, indent, leading, spacing, bullets.
func buildBraceParagraphStyleRequests(be *braceExpr, start, end int64) []*docs.Request {
	if be == nil {
		return nil
	}

	var requests []*docs.Request

	// Build paragraph style
	paraStyle := &docs.ParagraphStyle{}
	var paraFields []string

	// Heading
	if be.Heading != "" {
		namedStyle := resolveHeading(be.Heading)
		paraStyle.NamedStyleType = namedStyle
		paraFields = append(paraFields, "namedStyleType")
	}

	// Alignment
	if be.Align != "" {
		paraStyle.Alignment = resolveAlign(be.Align)
		paraFields = append(paraFields, "alignment")
	}

	// Indent level (converted to indentStart in points)
	if be.Indent >= 0 {
		// Each indent level = 36pt (standard Google Docs indent)
		indentPt := float64(be.Indent) * indentPointsPerLevel
		paraStyle.IndentStart = &docs.Dimension{Magnitude: indentPt, Unit: "PT"}
		paraFields = append(paraFields, "indentStart")
	}

	// Line height / leading
	if be.Leading > 0 {
		paraStyle.LineSpacing = be.Leading * 100 // 1.5 → 150
		paraFields = append(paraFields, "lineSpacing")
	}

	// Paragraph spacing (above/below)
	if be.SpacingSet {
		if be.SpacingAbove > 0 || be.SpacingBelow > 0 {
			paraStyle.SpaceAbove = &docs.Dimension{Magnitude: be.SpacingAbove, Unit: "PT"}
			paraStyle.SpaceBelow = &docs.Dimension{Magnitude: be.SpacingBelow, Unit: "PT"}
			paraFields = append(paraFields, "spaceAbove", "spaceBelow")
		} else {
			// Reset spacing to zero
			paraStyle.SpaceAbove = &docs.Dimension{Magnitude: 0, Unit: "PT"}
			paraStyle.SpaceBelow = &docs.Dimension{Magnitude: 0, Unit: "PT"}
			paraFields = append(paraFields, "spaceAbove", "spaceBelow")
		}
	}

	// Paragraph borders
	if be.BorderSet {
		borderColor := be.BorderColor
		if borderColor == "" {
			borderColor = "#000000" // default black
		}
		dashStyle := resolveBorderDashStyle(be.BorderStyle)

		makeBorder := func(widthPt float64) *docs.ParagraphBorder {
			if widthPt == 0 {
				// Zero width = remove border
				return &docs.ParagraphBorder{
					Width:     &docs.Dimension{Magnitude: 0, Unit: "PT"},
					DashStyle: "SOLID",
					Padding:   &docs.Dimension{Magnitude: 0, Unit: "PT"},
				}
			}
			return &docs.ParagraphBorder{
				Color:     hexColor(borderColor),
				Width:     &docs.Dimension{Magnitude: widthPt, Unit: "PT"},
				DashStyle: dashStyle,
				Padding:   &docs.Dimension{Magnitude: borderPaddingPt, Unit: "PT"},
			}
		}

		if be.BorderAll > 0 || (be.BorderAll == 0 && be.BorderTop == 0 && be.BorderBottom == 0 && be.BorderLeft == 0 && be.BorderRight == 0) {
			// d=N applies to all sides; d=0 removes all
			w := be.BorderAll
			paraStyle.BorderTop = makeBorder(w)
			paraStyle.BorderBottom = makeBorder(w)
			paraStyle.BorderLeft = makeBorder(w)
			paraStyle.BorderRight = makeBorder(w)
			paraFields = append(paraFields, "borderTop", "borderBottom", "borderLeft", "borderRight")
		}
		// Per-side overrides
		if be.BorderTop > 0 {
			paraStyle.BorderTop = makeBorder(be.BorderTop)
			paraFields = append(paraFields, "borderTop")
		}
		if be.BorderBottom > 0 {
			paraStyle.BorderBottom = makeBorder(be.BorderBottom)
			paraFields = append(paraFields, "borderBottom")
		}
		if be.BorderLeft > 0 {
			paraStyle.BorderLeft = makeBorder(be.BorderLeft)
			paraFields = append(paraFields, "borderLeft")
		}
		if be.BorderRight > 0 {
			paraStyle.BorderRight = makeBorder(be.BorderRight)
			paraFields = append(paraFields, "borderRight")
		}
	}

	if len(paraFields) > 0 {
		requests = append(requests, &docs.Request{
			UpdateParagraphStyle: &docs.UpdateParagraphStyleRequest{
				Range:          &docs.Range{StartIndex: start, EndIndex: end},
				ParagraphStyle: paraStyle,
				Fields:         strings.Join(paraFields, ","),
			},
		})
	}

	return requests
}

const borderPaddingPt = 4.0

// resolveBorderDashStyle maps user-friendly names to Google Docs API dash styles.
func resolveBorderDashStyle(s string) string {
	switch strings.ToLower(s) {
	case "dash", "dashed":
		return "DASH"
	case "dot", "dotted":
		return "DOT"
	case "dash_dot", "dashdot":
		return "DASH_DOT"
	case "long_dash", "longdash":
		return "LONG_DASH"
	case "long_dash_dot", "longdashdot":
		return "LONG_DASH_DOT"
	default:
		return "SOLID"
	}
}

// hexColor converts a hex color string to an OptionalColor.
func hexColor(hex string) *docs.OptionalColor {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return greyColor(0) // fallback to black
	}
	r, _ := strconv.ParseInt(hex[0:2], 16, 64)
	g, _ := strconv.ParseInt(hex[2:4], 16, 64)
	b, _ := strconv.ParseInt(hex[4:6], 16, 64)
	return &docs.OptionalColor{
		Color: &docs.Color{
			RgbColor: &docs.RgbColor{
				Red:   float64(r) / 255.0,
				Green: float64(g) / 255.0,
				Blue:  float64(b) / 255.0,
			},
		},
	}
}

// buildBraceInlineRequests handles inline scoping — multiple styled spans within one replacement.
// Each span has its own start/end positions and formatting.
func buildBraceInlineRequests(spans []*braceSpan, baseIndex int64) []*docs.Request {
	if len(spans) == 0 {
		return nil
	}

	var requests []*docs.Request
	for _, span := range spans {
		if span.IsGlobal || span.Expr == nil {
			continue // Global spans handled separately
		}

		start := baseIndex + int64(span.Start)
		end := baseIndex + int64(span.End)
		if end <= start {
			continue
		}

		requests = append(requests, buildBraceTextStyleRequests(span.Expr, start, end)...)
	}

	return requests
}

// buildBraceBreakRequests handles break flags (+, +=p, +=c, +=s).
// Returns requests to insert horizontal rules, page/column/section breaks.
func buildBraceBreakRequests(be *braceExpr, insertIdx int64) []*docs.Request {
	if be == nil || !be.HasBreak {
		return nil
	}

	var requests []*docs.Request

	switch be.Break {
	case "p": // Page break
		requests = append(requests, &docs.Request{
			InsertPageBreak: &docs.InsertPageBreakRequest{
				Location: &docs.Location{Index: insertIdx},
			},
		})
	case "c": // Column break
		// Column break is just a special character insert
		requests = append(requests, &docs.Request{
			InsertText: &docs.InsertTextRequest{
				Location: &docs.Location{Index: insertIdx},
				Text:     "\v", // Vertical tab = column break in Docs
			},
		})
	case "s": // Section break
		requests = append(requests, &docs.Request{
			InsertSectionBreak: &docs.InsertSectionBreakRequest{
				Location:    &docs.Location{Index: insertIdx},
				SectionType: "NEXT_PAGE",
			},
		})
	default: // "" = horizontal rule
		// Insert newline with bottom border
		requests = append(requests, &docs.Request{
			InsertText: &docs.InsertTextRequest{
				Location: &docs.Location{Index: insertIdx},
				Text:     "\n",
			},
		})
		requests = append(requests, buildHruleBorderRequest(insertIdx, insertIdx+1))
	}

	return requests
}

// braceExprToFormats converts a braceExpr to the existing format string slice
// for backward compatibility with existing code paths.
func braceExprToFormats(be *braceExpr) []string {
	if be == nil {
		return nil
	}

	var formats []string

	// Boolean flags
	if be.Bold != nil && *be.Bold {
		formats = append(formats, "bold")
	}
	if be.Italic != nil && *be.Italic {
		formats = append(formats, "italic")
	}
	if be.Underline != nil && *be.Underline {
		formats = append(formats, "underline")
	}
	if be.Strike != nil && *be.Strike {
		formats = append(formats, "strikethrough")
	}
	if be.Code != nil && *be.Code {
		formats = append(formats, "code")
	}
	if be.Sup != nil && *be.Sup {
		formats = append(formats, "superscript")
	}
	if be.Sub != nil && *be.Sub {
		formats = append(formats, "subscript")
	}
	if be.SmallCaps != nil && *be.SmallCaps {
		formats = append(formats, "smallcaps")
	}

	// Value flags
	if be.Font != "" {
		formats = append(formats, "font:"+be.Font)
	}
	if be.Size > 0 {
		formats = append(formats, "size:"+formatFloat(be.Size))
	}
	if be.Color != "" {
		formats = append(formats, "color:"+be.Color)
	}
	if be.Bg != "" {
		formats = append(formats, "bg:"+be.Bg)
	}
	if be.URL != "" {
		formats = append(formats, "link:"+be.URL)
	}

	// Heading
	if be.Heading != "" {
		level := be.Heading
		switch level {
		case "t":
			formats = append(formats, "title")
		case "s":
			formats = append(formats, "subtitle")
		case "0":
			formats = append(formats, "normal")
		default:
			formats = append(formats, "heading"+level)
		}
	}

	// Alignment
	if be.Align != "" {
		formats = append(formats, "align:"+be.Align)
	}

	return formats
}

// formatFloat formats a float64 without unnecessary trailing zeros.
func formatFloat(f float64) string {
	// Check if it's a whole number
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	// Format with precision, trim trailing zeros
	s := strconv.FormatFloat(f, 'f', -1, 64)
	return s
}

// hasBraceTextFormat checks if braceExpr has formatting that requires text styling.
func hasBraceTextFormat(be *braceExpr) bool {
	return braceExprHasTextFormat(be)
}

// hasBraceParagraphFormat checks if braceExpr has formatting that requires paragraph styling.
func hasBraceParagraphFormat(be *braceExpr) bool {
	if be == nil {
		return false
	}
	return be.Heading != "" || be.Align != "" || be.Indent >= 0 ||
		be.Leading > 0 || be.SpacingSet || be.BorderSet
}

// mergeBraceSpans merges multiple braceSpans into a single braceExpr for global formatting.
// Only global spans (those that apply to the entire match) are merged; non-global spans
// represent inline-scoped formatting (e.g., {b=Warning}) and are handled separately
// by buildBraceInlineRequests, which applies them at their specific positions.
func mergeBraceSpans(spans []*braceSpan) *braceExpr {
	merged := &braceExpr{Indent: indentNotSet}
	for _, span := range spans {
		if span.IsGlobal && span.Expr != nil {
			// Copy global flags to merged
			if span.Expr.Bold != nil {
				merged.Bold = span.Expr.Bold
			}
			if span.Expr.Italic != nil {
				merged.Italic = span.Expr.Italic
			}
			if span.Expr.Underline != nil {
				merged.Underline = span.Expr.Underline
			}
			if span.Expr.Strike != nil {
				merged.Strike = span.Expr.Strike
			}
			if span.Expr.Code != nil {
				merged.Code = span.Expr.Code
			}
			if span.Expr.Sup != nil {
				merged.Sup = span.Expr.Sup
			}
			if span.Expr.Sub != nil {
				merged.Sub = span.Expr.Sub
			}
			if span.Expr.SmallCaps != nil {
				merged.SmallCaps = span.Expr.SmallCaps
			}
			if span.Expr.Color != "" {
				merged.Color = span.Expr.Color
			}
			if span.Expr.Bg != "" {
				merged.Bg = span.Expr.Bg
			}
			if span.Expr.Font != "" {
				merged.Font = span.Expr.Font
			}
			if span.Expr.Size > 0 {
				merged.Size = span.Expr.Size
			}
			if span.Expr.URL != "" {
				merged.URL = span.Expr.URL
			}
			if span.Expr.Heading != "" {
				merged.Heading = span.Expr.Heading
			}
			if span.Expr.Align != "" {
				merged.Align = span.Expr.Align
			}
			if span.Expr.Leading > 0 {
				merged.Leading = span.Expr.Leading
			}
			if span.Expr.SpacingSet {
				merged.SpacingSet = true
				merged.SpacingAbove = span.Expr.SpacingAbove
				merged.SpacingBelow = span.Expr.SpacingBelow
			}
			if span.Expr.Indent >= 0 {
				merged.Indent = span.Expr.Indent
			}
			if span.Expr.Reset {
				merged.Reset = true
			}
			if span.Expr.HasBreak {
				merged.HasBreak = true
				merged.Break = span.Expr.Break
			}
		}
	}
	return merged
}

// (Legacy attrsTobraceExpr removed — sedAttrs no longer exists)
