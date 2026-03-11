// Package adf handles Atlassian Document Format (ADF) JSON rendering and editing.
package adf

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Node represents an ADF document node.
type Node struct {
	Type    string          `json:"type"`
	Content []Node          `json:"content,omitempty"`
	Text    string          `json:"text,omitempty"`
	Attrs   json.RawMessage `json:"attrs,omitempty"`
	Marks   []Mark          `json:"marks,omitempty"`
}

// Mark represents an inline text mark (bold, italic, code, link, etc.).
type Mark struct {
	Type  string          `json:"type"`
	Attrs json.RawMessage `json:"attrs,omitempty"`
}

// Render converts an ADF JSON document into a plain text string suitable
// for terminal display.
func Render(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}

	var node Node
	if err := json.Unmarshal(raw, &node); err != nil {
		return ""
	}
	return renderNode(&node, 0)
}

func renderNode(n *Node, depth int) string {
	switch n.Type {
	case "doc":
		return renderChildren(n.Content, depth)

	case "paragraph":
		return renderChildren(n.Content, depth) + "\n"

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
		return strings.Repeat("#", level) + " " + renderChildren(n.Content, depth) + "\n\n"

	case "text":
		return applyMarks(n.Text, n.Marks)

	case "hardBreak":
		return "\n"

	case "bulletList":
		return renderList(n.Content, depth, "•")

	case "orderedList":
		return renderOrderedList(n.Content, depth)

	case "listItem":
		return renderChildren(n.Content, depth)

	case "codeBlock":
		var attrs struct {
			Language string `json:"language"`
		}
		if n.Attrs != nil {
			_ = json.Unmarshal(n.Attrs, &attrs)
		}
		lang := ""
		if attrs.Language != "" {
			lang = " (" + attrs.Language + ")"
		}
		return "```" + lang + "\n" + renderChildren(n.Content, depth) + "```\n\n"

	case "blockquote":
		lines := strings.Split(strings.TrimRight(renderChildren(n.Content, depth), "\n"), "\n")
		var quoted []string
		for _, l := range lines {
			quoted = append(quoted, "  │ "+l)
		}
		return strings.Join(quoted, "\n") + "\n\n"

	case "rule":
		return strings.Repeat("─", 40) + "\n\n"

	case "table":
		return renderTable(n)

	case "panel":
		var attrs struct {
			PanelType string `json:"panelType"`
		}
		if n.Attrs != nil {
			_ = json.Unmarshal(n.Attrs, &attrs)
		}
		icon := "ℹ"
		switch attrs.PanelType {
		case "warning":
			icon = "⚠"
		case "error":
			icon = "✗"
		case "success":
			icon = "✓"
		case "note":
			icon = "📝"
		}
		return icon + " " + strings.TrimSpace(renderChildren(n.Content, depth)) + "\n\n"

	case "mention":
		var attrs struct {
			Text string `json:"text"`
		}
		if n.Attrs != nil {
			_ = json.Unmarshal(n.Attrs, &attrs)
		}
		if attrs.Text != "" {
			return "@" + attrs.Text
		}
		return "@mention"

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

	case "inlineCard":
		var attrs struct {
			URL string `json:"url"`
		}
		if n.Attrs != nil {
			_ = json.Unmarshal(n.Attrs, &attrs)
		}
		return attrs.URL

	case "mediaGroup", "mediaSingle", "media":
		return "[attachment]\n"

	default:
		return renderChildren(n.Content, depth)
	}
}

func renderChildren(nodes []Node, depth int) string {
	var sb strings.Builder
	for i := range nodes {
		sb.WriteString(renderNode(&nodes[i], depth))
	}
	return sb.String()
}

func renderList(items []Node, depth int, bullet string) string {
	indent := strings.Repeat("  ", depth)
	var sb strings.Builder
	for i := range items {
		content := strings.TrimRight(renderNode(&items[i], depth+1), "\n")
		lines := strings.SplitN(content, "\n", 2)
		sb.WriteString(indent + bullet + " " + lines[0] + "\n")
		if len(lines) > 1 && lines[1] != "" {
			sb.WriteString(lines[1])
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

func renderOrderedList(items []Node, depth int) string {
	indent := strings.Repeat("  ", depth)
	var sb strings.Builder
	for i := range items {
		content := strings.TrimRight(renderNode(&items[i], depth+1), "\n")
		lines := strings.SplitN(content, "\n", 2)
		sb.WriteString(indent + fmt.Sprintf("%d. ", i+1) + lines[0] + "\n")
		if len(lines) > 1 && lines[1] != "" {
			sb.WriteString(lines[1])
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

func renderTable(n *Node) string {
	var sb strings.Builder
	for _, row := range n.Content {
		if row.Type != "tableRow" {
			continue
		}
		var cells []string
		for _, cell := range row.Content {
			text := strings.TrimRight(renderChildren(cell.Content, 0), "\n")
			text = strings.ReplaceAll(text, "\n", " ")
			cells = append(cells, text)
		}
		sb.WriteString("  " + strings.Join(cells, "  │  ") + "\n")
	}
	sb.WriteString("\n")
	return sb.String()
}

func applyMarks(text string, marks []Mark) string {
	// Plain text display: return text as-is.
	// Marks are tracked for Markdown conversion in edit.go.
	return text
}
