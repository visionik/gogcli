package cmd

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf16"
)

// MarkdownToSedmatExprs converts markdown text into a list of sedmat expressions.
// Each expression is a sed-style s/$/replacement/ that appends content with formatting.
// The resulting expressions can be executed by the sedmat engine (DocsSedCmd.runBatch).
func MarkdownToSedmatExprs(markdown string) []string {
	lines := strings.Split(markdown, "\n")
	var exprs []string

	i := 0
	for i < len(lines) {
		line := lines[i]

		// Code blocks: collect all lines between ``` markers
		if strings.HasPrefix(line, "```") {
			var codeLines []string
			lang := strings.TrimPrefix(line, "```")
			_ = lang // language hint ignored for now
			i++
			for i < len(lines) && !strings.HasPrefix(lines[i], "```") {
				codeLines = append(codeLines, lines[i])
				i++
			}
			if i < len(lines) {
				i++ // skip closing ```
			}
			codeContent := strings.Join(codeLines, "\n")
			// Use fenced code block syntax that parseMarkdownReplacement handles
			exprs = append(exprs, `s/$/`+sedEscape("```\n"+codeContent+"\n```")+`/`)
			continue
		}

		// Empty lines — skip (Google Docs doesn't need explicit blank paragraphs typically)
		if strings.TrimSpace(line) == "" {
			i++
			continue
		}

		// Horizontal rule
		if isHorizontalRule(line) {
			exprs = append(exprs, `s/$/---/`)
			i++
			continue
		}

		// Table: starts with | and next line is separator
		if strings.HasPrefix(line, "|") && strings.Count(line, "|") >= 2 {
			if i+1 < len(lines) && isTableSeparator(lines[i+1]) {
				tableLines := collectTableLines(lines, i)
				pipeTable := buildSedmatPipeTable(tableLines)
				exprs = append(exprs, `s/$/`+sedEscape(pipeTable)+`/`)
				i += len(tableLines) + 1 // +1 for separator line
				// Skip any remaining table data rows already consumed
				for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "|") {
					i++
				}
				continue
			}
		}

		// Headings
		if level, content := parseHeading(line); level > 0 {
			// Convert inline formatting in heading content to brace syntax
			converted := convertInlineToBraces(content)
			exprs = append(exprs, fmt.Sprintf(`s/$/%s %s/`, strings.Repeat("#", level), sedEscape(converted)))
			i++
			continue
		}

		// Blockquote
		if strings.HasPrefix(line, "> ") {
			content := strings.TrimPrefix(line, "> ")
			converted := convertInlineToBraces(content)
			exprs = append(exprs, `s/$/> `+sedEscape(converted)+`/`)
			i++
			continue
		}

		// List items (unordered)
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			// Preserve indentation for nested lists
			indent := ""
			rest := line
			for strings.HasPrefix(rest, "  ") {
				indent += "  "
				rest = rest[2:]
			}
			var content string
			if strings.HasPrefix(rest, "- ") {
				content = rest[2:]
			} else if strings.HasPrefix(rest, "* ") {
				content = rest[2:]
			} else {
				content = rest
			}
			converted := convertInlineToBraces(content)
			exprs = append(exprs, `s/$/`+sedEscape(indent+"- "+converted)+`/`)
			i++
			continue
		}

		// Numbered list
		if match := regexp.MustCompile(`^(\d+)\.\s+(.+)`).FindStringSubmatch(line); match != nil {
			converted := convertInlineToBraces(match[2])
			exprs = append(exprs, `s/$/`+sedEscape(match[1]+". "+converted)+`/`)
			i++
			continue
		}

		// Regular paragraph — convert inline formatting to brace expressions
		converted := convertInlineToBraces(line)
		exprs = append(exprs, `s/$/`+sedEscape(converted)+`/`)
		i++
	}

	return exprs
}

// collectTableLines collects header + data rows (skipping separator) from a markdown table.
func collectTableLines(lines []string, start int) []string {
	var result []string
	for j := start; j < len(lines); j++ {
		line := strings.TrimSpace(lines[j])
		if line == "" || !strings.HasPrefix(line, "|") {
			break
		}
		if isTableSeparator(line) {
			continue
		}
		result = append(result, line)
	}
	return result
}

// buildSedmatPipeTable converts collected table rows into sedmat pipe-table syntax.
// Input rows are already in | cell | cell | format.
func buildSedmatPipeTable(rows []string) string {
	// The sedmat engine's parseTableFromPipes already handles pipe-table syntax directly.
	// Just join the rows with \n.
	return strings.Join(rows, "\n")
}

// convertInlineToBraces converts markdown inline formatting to sedmat brace expressions.
// For lines that are entirely one format (e.g., **all bold**), returns markdown syntax.
// For mixed inline formatting, converts to brace expressions.
func convertInlineToBraces(text string) string {
	// Check if there's any inline formatting at all
	if !hasInlineFormatting(text) {
		return text
	}

	// Check if the whole line is a single format — keep as markdown for parseMarkdownReplacement
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "***") && strings.HasSuffix(trimmed, "***") && strings.Count(trimmed, "***") == 2 {
		return text
	}
	if strings.HasPrefix(trimmed, "**") && strings.HasSuffix(trimmed, "**") && !strings.Contains(trimmed[2:len(trimmed)-2], "**") {
		return text
	}
	if strings.HasPrefix(trimmed, "*") && strings.HasSuffix(trimmed, "*") && !strings.Contains(trimmed[1:len(trimmed)-1], "*") {
		return text
	}
	if strings.HasPrefix(trimmed, "`") && strings.HasSuffix(trimmed, "`") && !strings.Contains(trimmed[1:len(trimmed)-1], "`") {
		return text
	}
	if strings.HasPrefix(trimmed, "~~") && strings.HasSuffix(trimmed, "~~") && !strings.Contains(trimmed[2:len(trimmed)-2], "~~") {
		return text
	}
	if strings.HasPrefix(trimmed, "[") && strings.Contains(trimmed, "](") && strings.HasSuffix(trimmed, ")") {
		return text
	}

	// Mixed inline formatting — convert to brace expressions
	return convertMixedInline(text)
}

// hasInlineFormatting returns true if text contains markdown inline formatting markers.
func hasInlineFormatting(text string) bool {
	return strings.Contains(text, "**") ||
		strings.Contains(text, "*") ||
		strings.Contains(text, "`") ||
		strings.Contains(text, "~~") ||
		strings.Contains(text, "](")
}

// inlineToken represents a parsed inline formatting token.
type inlineToken struct {
	text   string
	bold   bool
	italic bool
	code   bool
	strike bool
	link   string // URL for links
}

// convertMixedInline converts a line with mixed inline markdown formatting to brace expressions.
func convertMixedInline(text string) string {
	// Use a state-machine approach to parse inline formatting
	var result strings.Builder
	i := 0

	for i < len(text) {
		// Link: [text](url)
		if text[i] == '[' {
			if end, linkText, url := parseInlineLink(text, i); end > i {
				innerConverted := convertMixedInline(linkText)
				result.WriteString(fmt.Sprintf("{u=%s %s}", sedEscapeBrace(url), innerConverted))
				i = end
				continue
			}
		}

		// Code: `text`
		if text[i] == '`' {
			if end, content := parseInlineCode(text, i); end > i {
				result.WriteString(fmt.Sprintf("{#=%s}", sedEscapeBrace(content)))
				i = end
				continue
			}
		}

		// Strikethrough: ~~text~~
		if i+1 < len(text) && text[i] == '~' && text[i+1] == '~' {
			if end, content := parseInlineStrike(text, i); end > i {
				innerConverted := convertMixedInline(content)
				result.WriteString(fmt.Sprintf("{-=%s}", innerConverted))
				i = end
				continue
			}
		}

		// Bold+italic: ***text***
		if i+2 < len(text) && text[i] == '*' && text[i+1] == '*' && text[i+2] == '*' {
			if end, content := parseInlineDelimited(text, i, "***"); end > i {
				innerConverted := convertMixedInline(content)
				result.WriteString(fmt.Sprintf("{b i=%s}", innerConverted))
				i = end
				continue
			}
		}

		// Bold: **text**
		if i+1 < len(text) && text[i] == '*' && text[i+1] == '*' {
			if end, content := parseInlineDelimited(text, i, "**"); end > i {
				innerConverted := convertMixedInline(content)
				result.WriteString(fmt.Sprintf("{b=%s}", innerConverted))
				i = end
				continue
			}
		}

		// Italic: *text*
		if text[i] == '*' {
			if end, content := parseInlineDelimited(text, i, "*"); end > i {
				innerConverted := convertMixedInline(content)
				result.WriteString(fmt.Sprintf("{i=%s}", innerConverted))
				i = end
				continue
			}
		}

		// Plain text
		result.WriteByte(text[i])
		i++
	}

	return result.String()
}

// parseInlineLink parses [text](url) starting at position i.
// Returns end position (past closing ')'), link text, and URL.
func parseInlineLink(text string, i int) (int, string, string) {
	if text[i] != '[' {
		return i, "", ""
	}
	// Find closing ]
	closeIdx := -1
	for j := i + 1; j < len(text); j++ {
		if text[j] == ']' {
			closeIdx = j
			break
		}
	}
	if closeIdx < 0 || closeIdx+1 >= len(text) || text[closeIdx+1] != '(' {
		return i, "", ""
	}
	// Find closing )
	parenClose := -1
	for j := closeIdx + 2; j < len(text); j++ {
		if text[j] == ')' {
			parenClose = j
			break
		}
	}
	if parenClose < 0 {
		return i, "", ""
	}
	linkText := text[i+1 : closeIdx]
	url := text[closeIdx+2 : parenClose]
	return parenClose + 1, linkText, url
}

// parseInlineCode parses `text` starting at position i.
func parseInlineCode(text string, i int) (int, string) {
	if text[i] != '`' {
		return i, ""
	}
	closeIdx := strings.Index(text[i+1:], "`")
	if closeIdx < 0 {
		return i, ""
	}
	return i + 1 + closeIdx + 1, text[i+1 : i+1+closeIdx]
}

// parseInlineStrike parses ~~text~~ starting at position i.
func parseInlineStrike(text string, i int) (int, string) {
	if i+1 >= len(text) || text[i] != '~' || text[i+1] != '~' {
		return i, ""
	}
	closeIdx := strings.Index(text[i+2:], "~~")
	if closeIdx < 0 {
		return i, ""
	}
	return i + 2 + closeIdx + 2, text[i+2 : i+2+closeIdx]
}

// parseInlineDelimited parses text delimited by delimiter (e.g., "**", "*", "***").
func parseInlineDelimited(text string, i int, delimiter string) (int, string) {
	dl := len(delimiter)
	if i+dl > len(text) || text[i:i+dl] != delimiter {
		return i, ""
	}
	rest := text[i+dl:]
	closeIdx := strings.Index(rest, delimiter)
	if closeIdx < 0 {
		return i, ""
	}
	return i + dl + closeIdx + dl, rest[:closeIdx]
}

// sedEscape escapes characters that are special in sed replacement strings.
// Forward slashes need escaping since they're the delimiter.
func sedEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `/`, `\/`)
	return s
}

// sedEscapeBrace escapes characters that would be problematic inside brace expressions.
func sedEscapeBrace(s string) string {
	// Inside {flag=text}, we need to be careful with } and spaces
	// The parseBraceExpr handles most content, but we escape / for sed delimiter
	s = strings.ReplaceAll(s, `/`, `\/`)
	return s
}

// debugMarkdown enables verbose debug output for markdown processing.
var debugMarkdown = false

// inlineTypeCode is the type string for inline code formatting.
const inlineTypeCode = "code"

// --- Utility functions formerly in docs_markdown.go, retained for shared use ---

// utf16Len returns the number of UTF-16 code units in a string.
// Used by docs_table_inserter.go and other code that interacts with Google Docs indices.
func utf16Len(s string) int64 {
	return int64(len(utf16.Encode([]rune(s))))
}

// parseHeading parses a markdown heading line, returning the level (1-6) and content.
func parseHeading(line string) (int, string) {
	headingRegex := regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	match := headingRegex.FindStringSubmatch(line)
	if match == nil {
		return 0, ""
	}
	return len(match[1]), match[2]
}

// isHorizontalRule returns true if the line is a markdown horizontal rule (---, ***, ___).
func isHorizontalRule(line string) bool {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) < 3 {
		return false
	}
	char := trimmed[0]
	if char != '-' && char != '*' && char != '_' {
		return false
	}
	for _, c := range trimmed {
		if c != rune(char) && c != ' ' {
			return false
		}
	}
	return true
}

// isTableSeparator checks if a line is a markdown table separator (|---|---|).
func isTableSeparator(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") {
		return false
	}
	inner := strings.Trim(trimmed, "|")
	segments := strings.Split(inner, "|")
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		for i, c := range seg {
			if c != '-' && c != ' ' && c != ':' {
				return false
			}
			if c == ':' && i != 0 && i != len(seg)-1 {
				return false
			}
		}
		if strings.Count(seg, "-") == 0 {
			return false
		}
	}
	return len(segments) > 1
}
