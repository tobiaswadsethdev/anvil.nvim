package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tobiaswadsethdev/anvil.nvim/cmd/jira-anvil/api"
	"github.com/tobiaswadsethdev/anvil.nvim/cmd/jira-anvil/config"
	"github.com/tobiaswadsethdev/anvil.nvim/cmd/jira-anvil/ui"
)

var version = "dev"

func main() {
	var (
		configPath  = flag.String("config", "", "path to config file (default: ~/.config/anvil/config.yaml)")
		showVersion = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("jira-anvil %s\n", version)
		os.Exit(0)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "anvil: config error: %v\n\n", err)
		fmt.Fprintf(os.Stderr, "Create ~/.config/anvil/config.yaml:\n\n")
		fmt.Fprintf(os.Stderr, "  jira:\n")
		fmt.Fprintf(os.Stderr, "    url: https://yourcompany.atlassian.net\n")
		fmt.Fprintf(os.Stderr, "    user: you@example.com\n")
		fmt.Fprintf(os.Stderr, "    token: your-api-token\n")
		fmt.Fprintf(os.Stderr, "  filters:\n")
		fmt.Fprintf(os.Stderr, "    - name: My Issues\n")
		fmt.Fprintf(os.Stderr, "      jql: assignee = currentUser() ORDER BY updated DESC\n\n")
		fmt.Fprintf(os.Stderr, "Or use JIRA_URL, JIRA_USER, JIRA_TOKEN environment variables.\n")
		os.Exit(1)
	}

	client := api.NewClient(cfg.Jira.URL, cfg.Jira.User, cfg.Jira.Token)

	model := ui.NewModel(cfg, client)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "anvil: %v\n", err)
		os.Exit(1)
	}
}
