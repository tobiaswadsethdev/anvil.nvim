# anvil.nvim

A Jira Cloud TUI for Neovim — browse and administer Jira issues via named JQL filters, integrated as a floating terminal via [snacks.nvim](https://github.com/folke/snacks.nvim) (the same pattern as lazygit).

## Features

- **JQL filter cycling** — define named filters and cycle through them with `[` / `]`
- **Issue list** — table view with key, summary, status, priority, assignee, age
- **Issue detail** — scrollable view with ADF description rendering, comments, custom fields
- **Transitions** — change issue status via modal
- **Comments** — add comments (Markdown → ADF)
- **Assign** — fuzzy-search and assign users
- **Edit** — edit description and ADF custom fields in `$EDITOR`
- **Browser** — open any issue in browser with `o`

## Requirements

- Neovim ≥ 0.9
- [snacks.nvim](https://github.com/folke/snacks.nvim)
- Go ≥ 1.21 (to build `jira-anvil`)
- Jira Cloud account + API token

## Installation

### 1. Install the plugin

**lazy.nvim:**

```lua
{
  "tobiaswadsethdev/anvil.nvim",
  build = "make install",
  dependencies = { "folke/snacks.nvim" },
  opts = {
    jira = {
      url   = "https://yourcompany.atlassian.net",
      user  = "you@example.com",
      token = vim.env.JIRA_API_TOKEN,  -- keep secrets in env, not config
    },
    filters = {
      { name = "My Issues",  jql = "assignee = currentUser() AND status != Done ORDER BY updated DESC" },
      { name = "My Sprint",  jql = "project = PROJ AND sprint in openSprints() ORDER BY priority ASC" },
    },
  },
}
```

### 2. Build the binary

```sh
# Option A: via Makefile (recommended)
make install

# Option B: direct go install
cd cmd/jira-anvil && go install .
```

Ensure `~/go/bin` is in your `PATH`.

### 3. Configure

```lua
require('anvil').setup({
  jira = {
    url   = "https://yourcompany.atlassian.net",
    user  = "you@example.com",
    token = vim.env.JIRA_API_TOKEN,
  },
  filters = {
    { name = "My Issues",   jql = "assignee = currentUser() AND status != Done ORDER BY updated DESC" },
    { name = "Sprint",      jql = "project = PROJ AND sprint in openSprints() ORDER BY priority ASC" },
    { name = "Unassigned",  jql = "project = PROJ AND assignee is EMPTY AND status = 'To Do'" },
  },
  keymaps = {
    open   = "<leader>jj",
    toggle = "<leader>jt",
  },
})
```

Alternatively set environment variables: `JIRA_URL`, `JIRA_USER`, `JIRA_TOKEN`.

## Usage

| Key / Command     | Action                    |
|-------------------|---------------------------|
| `<leader>jj`      | Open Jira TUI             |
| `<leader>jt`      | Toggle Jira TUI           |
| `:Anvil`          | Open Jira TUI             |
| `:AnvilToggle`    | Toggle Jira TUI           |
| `:checkhealth anvil` | Verify setup           |

## TUI Keybindings

### Issue List
| Key    | Action                  |
|--------|-------------------------|
| `[`    | Previous filter         |
| `]`    | Next filter             |
| `↑/k`  | Move up                 |
| `↓/j`  | Move down               |
| `Enter`| Open issue detail       |
| `r`    | Refresh                 |
| `o`    | Open in browser         |
| `q`    | Quit                    |

### Issue Detail
| Key    | Action                  |
|--------|-------------------------|
| `↑/k`  | Scroll up               |
| `↓/j`  | Scroll down             |
| `t`    | Transition status       |
| `c`    | Add comment             |
| `a`    | Assign issue            |
| `e`    | Edit in `$EDITOR`       |
| `o`    | Open in browser         |
| `q/Esc`| Back to list            |

## Health Check

```
:checkhealth anvil
```

## ADF Support

Jira uses [Atlassian Document Format (ADF)](https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/) for rich text. anvil.nvim:

- **Renders** ADF descriptions, comments, and custom fields to readable plain text
- **Edits** via `ADF → Markdown → $EDITOR → Markdown → ADF` round-trip
- Supports: paragraphs, headings, lists, code blocks, blockquotes, tables, mentions, inline marks

## Future: Azure DevOps

Azure DevOps pull requests and pipeline support is planned as `azdo-anvil` — same snacks.nvim integration pattern. A top-level `azdo:` section will be added to the config.

## License

Apache 2.0
