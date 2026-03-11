package ui

import "github.com/charmbracelet/lipgloss"

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

// TruncateString truncates a string to maxLen, adding ellipsis if needed.
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
