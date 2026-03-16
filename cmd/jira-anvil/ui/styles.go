package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Base colors
	colorPrimary  = lipgloss.Color("12")  // bright blue
	colorSecond   = lipgloss.Color("14")  // bright cyan
	colorMuted    = lipgloss.Color("8")   // dark gray
	colorAccent   = lipgloss.Color("205") // pink
	colorGreen    = lipgloss.Color("10")  // bright green
	colorYellow   = lipgloss.Color("11")  // bright yellow
	colorRed      = lipgloss.Color("9")   // bright red
	colorFg       = lipgloss.Color("15")  // white
	colorBg       = lipgloss.Color("0")   // black
	colorSelected = lipgloss.Color("4")   // blue (selected row bg)

	// Status styles
	statusDone       = lipgloss.NewStyle().Foreground(colorGreen)
	statusInProgress = lipgloss.NewStyle().Foreground(colorPrimary)
	statusTodo       = lipgloss.NewStyle().Foreground(colorMuted)
	statusBlocked    = lipgloss.NewStyle().Foreground(colorRed)

	// Priority styles
	priorityCritical = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	priorityHigh     = lipgloss.NewStyle().Foreground(colorRed)
	priorityMedium   = lipgloss.NewStyle().Foreground(colorYellow)
	priorityLow      = lipgloss.NewStyle().Foreground(colorMuted)

	// Layout styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			Padding(0, 1)

	filterBarStyle = lipgloss.NewStyle().
			Background(colorSelected).
			Foreground(colorFg).
			Padding(0, 1).
			Bold(true)

	statusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("237")).
			Foreground(colorMuted).
			Padding(0, 1)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true).
			Padding(1, 2)

	// Modal styles
	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(1, 2)

	modalTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			MarginBottom(1)

	selectedItemStyle = lipgloss.NewStyle().
				Background(colorSelected).
				Foreground(colorFg).
				Padding(0, 1)

	normalItemStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// Detail view
	fieldLabelStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Width(14)

	fieldValueStyle = lipgloss.NewStyle().
			Foreground(colorFg)

	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSecond).
			MarginTop(1).
			MarginBottom(1)

	commentMetaStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Italic(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Padding(0, 1)

	keyStyle = lipgloss.NewStyle().
			Foreground(colorSecond).
			Bold(true)

	// Panel styles (lazygit-style bordered boxes)
	panelActiveStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorAccent).
				Padding(0, 1)

	panelInactiveStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorMuted).
				Padding(0, 1)

	panelTitleActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorAccent)

	panelTitleInactiveStyle = lipgloss.NewStyle().
				Foreground(colorMuted)

	panelTabActiveStyle = lipgloss.NewStyle().
				Background(colorSelected).
				Foreground(colorFg).
				Bold(true).
				Padding(0, 1)

	panelTabInactiveStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Padding(0, 1)
)

// ColorStatus returns a styled status string.
func ColorStatus(status string) string {
	switch status {
	case "Done", "Closed", "Resolved", "Complete":
		return statusDone.Render(status)
	case "In Progress", "In Review", "In Development":
		return statusInProgress.Render(status)
	case "Blocked", "Impediment":
		return statusBlocked.Render(status)
	default:
		return statusTodo.Render(status)
	}
}

// ColorPriority returns a styled priority string with icon.
func ColorPriority(priority string) string {
	switch priority {
	case "Critical", "Blocker":
		return priorityCritical.Render("▲▲ " + priority)
	case "High":
		return priorityHigh.Render("▲ " + priority)
	case "Medium":
		return priorityMedium.Render("● " + priority)
	case "Low", "Trivial":
		return priorityLow.Render("▼ " + priority)
	default:
		return priority
	}
}

// renderPanelTitle renders a "[N]-Title" header line for a panel.
func renderPanelTitle(num int, title string, active bool) string {
	label := fmt.Sprintf("[%d]-%s", num, title)
	if active {
		return panelTitleActiveStyle.Render(label)
	}
	return panelTitleInactiveStyle.Render(label)
}

// renderPanelTabs renders a tab bar for panels that have multiple tabs.
func renderPanelTabs(tabs []string, activeTab int, width int) string {
	var parts []string
	for i, tab := range tabs {
		if i == activeTab {
			parts = append(parts, panelTabActiveStyle.Render(tab))
		} else {
			parts = append(parts, panelTabInactiveStyle.Render(tab))
		}
	}
	bar := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	barW := lipgloss.Width(bar)
	if width > barW {
		filler := lipgloss.NewStyle().Render(strings.Repeat(" ", width-barW))
		bar += filler
	}
	return bar
}

// wrapPanel wraps content in an active or inactive bordered panel of the given outer width/height.
func wrapPanel(content string, active bool, outerW, outerH int) string {
	style := panelInactiveStyle
	if active {
		style = panelActiveStyle
	}
	// Inner dimensions: subtract 2 for borders + 2 for padding on each side horizontally, 2 for borders vertically
	innerW := outerW - 4
	innerH := outerH - 2
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}
	return style.Width(innerW).Height(innerH).Render(content)
}

func panelInnerSize(outerW, outerH int) (innerW, innerH int) {
	frameW := panelInactiveStyle.GetHorizontalFrameSize()
	frameH := panelInactiveStyle.GetVerticalFrameSize()
	innerW = maxInt(1, outerW-frameW)
	innerH = maxInt(1, outerH-frameH)
	return
}

func panelDivider(innerW int) string {
	return strings.Repeat("─", maxInt(1, innerW-2))
}

func renderPanelScaffold(num int, title string, active bool, tabs []string, activeTab int, innerW int, body string) string {
	parts := []string{renderPanelTitle(num, title, active)}
	if len(tabs) > 0 {
		parts = append(parts, renderPanelTabs(tabs, activeTab, innerW))
	}
	parts = append(parts, panelDivider(innerW))
	parts = append(parts, body)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// TruncateString truncates a string to maxLen, adding ellipsis if needed.
func TruncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
