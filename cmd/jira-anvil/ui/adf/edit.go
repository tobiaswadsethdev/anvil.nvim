package adf

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ToMarkdown converts an ADF JSON document to a Markdown string for editing.
func ToMarkdown(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	var node Node
	if err := json.Unmarshal(raw, &node); err != nil {
		return ""
	}
	return nodeToMarkdown(&node, 0)
}

// FromMarkdown converts a Markdown string back to an ADF JSON document.
func FromMarkdown(md string) json.RawMessage {
	doc := markdownToADF(md)
	data, err := json.Marshal(doc)
	if err != nil {
		// Fallback: wrap in simple paragraph
		data, _ = json.Marshal(simpleDoc(md))
	}
	return data
}

// --- ADF → Markdown ---

func nodeToMarkdown(n *Node, depth int) string {
	switch n.Type {
	case "doc":
		return childrenToMarkdown(n.Content, depth)

	case "paragraph":
		return childrenToMarkdown(n.Content, depth) + "\n\n"

	case "heading":
		level := 1
		if n.Attrs != nil {
			var attrs struct {
				Level int `json:"level"`
			}
			_ = json.Unmarshal(n.Attrs, &attrs)
			if attrs.Level > 0 {
				level = attrs.Level
			}
		}
		return strings.Repeat("#", level) + " " + childrenToMarkdown(n.Content, depth) + "\n\n"

	case "text":
		return textToMarkdown(n.Text, n.Marks)

	case "hardBreak":
		return "  \n"

	case "bulletList":
		return listToMarkdown(n.Content, depth, "-")

	case "orderedList":
		return orderedListToMarkdown(n.Content, depth)

	case "listItem":
		content := strings.TrimRight(childrenToMarkdown(n.Content, depth), "\n")
		return content

	case "codeBlock":
		var attrs struct {
			Language string `json:"language"`
		}
		if n.Attrs != nil {
			_ = json.Unmarshal(n.Attrs, &attrs)
		}
		return "```" + attrs.Language + "\n" + childrenToMarkdown(n.Content, depth) + "```\n\n"

	case "blockquote":
		lines := strings.Split(strings.TrimRight(childrenToMarkdown(n.Content, depth), "\n"), "\n")
		var quoted []string
		for _, l := range lines {
			quoted = append(quoted, "> "+l)
		}
		return strings.Join(quoted, "\n") + "\n\n"

	case "rule":
		return "---\n\n"

	case "table":
		return tableToMarkdown(n)

	case "panel":
		content := strings.TrimSpace(childrenToMarkdown(n.Content, depth))
		return "> **Note:** " + content + "\n\n"

	case "mention":
		var attrs struct {
			Text string `json:"text"`
		}
		if n.Attrs != nil {
			_ = json.Unmarshal(n.Attrs, &attrs)
		}
		return "@" + attrs.Text

	case "emoji":
		var attrs struct {
			ShortName string `json:"shortName"`
			Text      string `json:"text"`
		}
		if n.Attrs != nil {
			_ = json.Unmarshal(n.Attrs, &attrs)
		}
		if attrs.Text != "" {
			return attrs.Text
		}
		return attrs.ShortName

	case "mediaGroup", "mediaSingle", "media":
		return "[attachment]\n\n"

	default:
		return childrenToMarkdown(n.Content, depth)
	}
}

func childrenToMarkdown(nodes []Node, depth int) string {
	var sb strings.Builder
	for i := range nodes {
		sb.WriteString(nodeToMarkdown(&nodes[i], depth))
	}
	return sb.String()
}

func textToMarkdown(text string, marks []Mark) string {
	result := text
	for _, mark := range marks {
		switch mark.Type {
		case "strong":
			result = "**" + result + "**"
		case "em":
			result = "_" + result + "_"
		case "code":
			result = "`" + result + "`"
		case "strike":
			result = "~~" + result + "~~"
		case "link":
			var attrs struct {
				Href string `json:"href"`
			}
			if mark.Attrs != nil {
				_ = json.Unmarshal(mark.Attrs, &attrs)
			}
			result = "[" + result + "](" + attrs.Href + ")"
		}
	}
	return result
}

func listToMarkdown(items []Node, depth int, bullet string) string {
	indent := strings.Repeat("  ", depth)
	var sb strings.Builder
	for i := range items {
		content := strings.TrimRight(nodeToMarkdown(&items[i], depth+1), "\n")
		lines := strings.Split(content, "\n")
		for j, line := range lines {
			if j == 0 {
				sb.WriteString(indent + bullet + " " + line + "\n")
			} else if line != "" {
				sb.WriteString(indent + "  " + line + "\n")
			}
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

func orderedListToMarkdown(items []Node, depth int) string {
	indent := strings.Repeat("  ", depth)
	var sb strings.Builder
	for i := range items {
		content := strings.TrimRight(nodeToMarkdown(&items[i], depth+1), "\n")
		lines := strings.Split(content, "\n")
		for j, line := range lines {
			if j == 0 {
				sb.WriteString(fmt.Sprintf("%s%d. %s\n", indent, i+1, line))
			} else if line != "" {
				sb.WriteString(indent + "   " + line + "\n")
			}
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

func tableToMarkdown(n *Node) string {
	var sb strings.Builder
	headerDone := false
	for _, row := range n.Content {
		if row.Type != "tableRow" {
			continue
		}
		var cells []string
		isHeader := false
		for _, cell := range row.Content {
			if cell.Type == "tableHeader" {
				isHeader = true
			}
			text := strings.TrimRight(childrenToMarkdown(cell.Content, 0), "\n")
			text = strings.ReplaceAll(text, "\n", " ")
			cells = append(cells, text)
		}
		sb.WriteString("| " + strings.Join(cells, " | ") + " |\n")
		if (isHeader || !headerDone) && len(cells) > 0 {
			var sep []string
			for range cells {
				sep = append(sep, "---")
			}
			sb.WriteString("| " + strings.Join(sep, " | ") + " |\n")
			headerDone = true
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

// --- Markdown → ADF ---

var (
	reHeading    = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	reHR         = regexp.MustCompile(`^---+$`)
	reULItem     = regexp.MustCompile(`^(\s*)([-*+])\s+(.+)$`)
	reOLItem     = regexp.MustCompile(`^(\s*)\d+\.\s+(.+)$`)
	reCodeFence  = regexp.MustCompile("^```(\\w*)$")
	reCodeEnd    = regexp.MustCompile("^```$")
	reBlockquote = regexp.MustCompile(`^>\s?(.*)$`)
	reTableRow   = regexp.MustCompile(`^\|(.+)\|$`)
	reTableSep   = regexp.MustCompile(`^\|[-| :]+\|$`)
)

func markdownToADF(md string) Node {
	lines := strings.Split(md, "\n")
	doc := Node{Type: "doc", Version: 1, Content: []Node{}}

	i := 0
	for i < len(lines) {
		line := lines[i]

		// Heading
		if m := reHeading.FindStringSubmatch(line); m != nil {
			level := len(m[1])
			doc.Content = append(doc.Content, headingNode(m[2], level))
			i++
			continue
		}

		// Horizontal rule
		if reHR.MatchString(line) {
			doc.Content = append(doc.Content, Node{Type: "rule"})
			i++
			continue
		}

		// Code fence
		if m := reCodeFence.FindStringSubmatch(line); m != nil {
			lang := m[1]
			i++
			var codeLines []string
			for i < len(lines) && !reCodeEnd.MatchString(lines[i]) {
				codeLines = append(codeLines, lines[i])
				i++
			}
			if i < len(lines) {
				i++ // skip closing ```
			}
			doc.Content = append(doc.Content, codeBlockNode(strings.Join(codeLines, "\n"), lang))
			continue
		}

		// Blockquote
		if reBlockquote.MatchString(line) {
			var bqLines []string
			for i < len(lines) && reBlockquote.MatchString(lines[i]) {
				m := reBlockquote.FindStringSubmatch(lines[i])
				bqLines = append(bqLines, m[1])
				i++
			}
			inner := markdownToADF(strings.Join(bqLines, "\n"))
			doc.Content = append(doc.Content, Node{Type: "blockquote", Content: inner.Content})
			continue
		}

		// Table
		if reTableRow.MatchString(line) {
			var tableLines []string
			for i < len(lines) && reTableRow.MatchString(lines[i]) {
				tableLines = append(tableLines, lines[i])
				i++
			}
			doc.Content = append(doc.Content, tableNode(tableLines))
			continue
		}

		// Unordered list
		if reULItem.MatchString(line) {
			var listItems []Node
			for i < len(lines) && reULItem.MatchString(lines[i]) {
				m := reULItem.FindStringSubmatch(lines[i])
				listItems = append(listItems, listItemNode(m[3]))
				i++
			}
			doc.Content = append(doc.Content, Node{Type: "bulletList", Content: listItems})
			continue
		}

		// Ordered list
		if reOLItem.MatchString(line) {
			var listItems []Node
			for i < len(lines) && reOLItem.MatchString(lines[i]) {
				m := reOLItem.FindStringSubmatch(lines[i])
				listItems = append(listItems, listItemNode(m[2]))
				i++
			}
			doc.Content = append(doc.Content, Node{Type: "orderedList", Content: listItems})
			continue
		}

		// Empty line
		if strings.TrimSpace(line) == "" {
			i++
			continue
		}

		// Paragraph: collect consecutive non-special lines
		var paraLines []string
		for i < len(lines) {
			l := lines[i]
			if strings.TrimSpace(l) == "" {
				break
			}
			if reHeading.MatchString(l) || reHR.MatchString(l) ||
				reCodeFence.MatchString(l) || reBlockquote.MatchString(l) ||
				reTableRow.MatchString(l) || reULItem.MatchString(l) || reOLItem.MatchString(l) {
				break
			}
			paraLines = append(paraLines, l)
			i++
		}
		if len(paraLines) > 0 {
			doc.Content = append(doc.Content, paragraphNode(strings.Join(paraLines, " ")))
		}
	}

	return doc
}

func headingNode(text string, level int) Node {
	attrs, _ := json.Marshal(map[string]int{"level": level})
	return Node{
		Type:    "heading",
		Attrs:   attrs,
		Content: []Node{textNode(text)},
	}
}

func paragraphNode(text string) Node {
	return Node{
		Type:    "paragraph",
		Content: parseInline(text),
	}
}

func codeBlockNode(code, lang string) Node {
	var attrsRaw json.RawMessage
	if lang != "" {
		attrsRaw, _ = json.Marshal(map[string]string{"language": lang})
	}
	return Node{
		Type:  "codeBlock",
		Attrs: attrsRaw,
		Content: []Node{
			{Type: "text", Text: code},
		},
	}
}

func listItemNode(text string) Node {
	return Node{
		Type:    "listItem",
		Content: []Node{paragraphNode(text)},
	}
}

func tableNode(lines []string) Node {
	table := Node{Type: "table", Content: []Node{}}
	for _, line := range lines {
		if reTableSep.MatchString(line) {
			continue // skip separator row
		}
		line = strings.Trim(line, "|")
		cells := strings.Split(line, "|")
		row := Node{Type: "tableRow", Content: []Node{}}
		for _, cell := range cells {
			cell = strings.TrimSpace(cell)
			row.Content = append(row.Content, Node{
				Type:    "tableCell",
				Content: []Node{paragraphNode(cell)},
			})
		}
		table.Content = append(table.Content, row)
	}
	return table
}

func textNode(text string) Node {
	return Node{Type: "text", Text: text}
}

// parseInline parses inline Markdown (bold, italic, code, links) into ADF text nodes.
var (
	reBold   = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reItalic = regexp.MustCompile(`_(.+?)_`)
	reCode   = regexp.MustCompile("`(.+?)`")
	reLink   = regexp.MustCompile(`\[(.+?)\]\((.+?)\)`)
	reStrike = regexp.MustCompile(`~~(.+?)~~`)
)

func parseInline(text string) []Node {
	// Simple inline parser: find the first match of any pattern and split around it.
	type match struct {
		start, end int
		node       Node
	}

	findFirst := func(t string) *match {
		best := -1
		var bestMatch *match

		check := func(re *regexp.Regexp, makeNode func([]string) Node) {
			m := re.FindStringSubmatchIndex(t)
			if m != nil && (best == -1 || m[0] < best) {
				best = m[0]
				groups := re.FindStringSubmatch(t)
				bestMatch = &match{
					start: m[0],
					end:   m[1],
					node:  makeNode(groups),
				}
			}
		}

		check(reLink, func(g []string) Node {
			attrs, _ := json.Marshal(map[string]string{"href": g[2]})
			return Node{Type: "text", Text: g[1], Marks: []Mark{{Type: "link", Attrs: attrs}}}
		})
		check(reBold, func(g []string) Node {
			return Node{Type: "text", Text: g[1], Marks: []Mark{{Type: "strong"}}}
		})
		check(reItalic, func(g []string) Node {
			return Node{Type: "text", Text: g[1], Marks: []Mark{{Type: "em"}}}
		})
		check(reCode, func(g []string) Node {
			return Node{Type: "text", Text: g[1], Marks: []Mark{{Type: "code"}}}
		})
		check(reStrike, func(g []string) Node {
			return Node{Type: "text", Text: g[1], Marks: []Mark{{Type: "strike"}}}
		})

		return bestMatch
	}

	var nodes []Node
	remaining := text
	for remaining != "" {
		m := findFirst(remaining)
		if m == nil {
			if remaining != "" {
				nodes = append(nodes, textNode(remaining))
			}
			break
		}
		if m.start > 0 {
			nodes = append(nodes, textNode(remaining[:m.start]))
		}
		nodes = append(nodes, m.node)
		remaining = remaining[m.end:]
	}
	return nodes
}

func simpleDoc(text string) Node {
	return Node{
		Type:    "doc",
		Version: 1,
		Content: []Node{
			{
				Type:    "paragraph",
				Content: []Node{textNode(text)},
			},
		},
	}
}
