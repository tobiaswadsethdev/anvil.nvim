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
- **Azure DevOps PR tab** — linked pull request with git diff and pipeline status, displayed as a tab alongside the Jira issue
- **PR voting** — view reviewer votes and cast your own vote (Approve / Reject / etc.) directly from the PR tab with `v`

## Requirements

- Neovim ≥ 0.9
- [snacks.nvim](https://github.com/folke/snacks.nvim)
- Go ≥ 1.21 (to build `jira-anvil`)
- Jira Cloud account + API token
- Azure DevOps Personal Access Token *(optional, for PR tab)*

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
  -- Optional: Azure DevOps integration for PR tab in issue detail
  azdo = {
    url     = "https://dev.azure.com/myorg",
    project = "myproject",
    repo    = "myrepo",
    token   = vim.env.AZDO_TOKEN,
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

Alternatively set environment variables: `JIRA_URL`, `JIRA_USER`, `JIRA_TOKEN`, `AZDO_URL`, `AZDO_PROJECT`, `AZDO_REPO`, `AZDO_TOKEN`.

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
| `[`    | Previous tab (Jira / Pull Request) |
| `]`    | Next tab (Jira / Pull Request)     |
| `↑/k`  | Scroll up               |
| `↓/j`  | Scroll down             |
| `t`    | Transition status       |
| `c`    | Add comment             |
| `a`    | Assign issue            |
| `e`    | Edit in `$EDITOR`       |
| `v`    | Vote on PR (PR tab only)|
| `o`    | Open in browser         |
| `q/Esc`| Back to list            |

## Azure DevOps PR Tab

When Azure DevOps is configured, opening any issue detail automatically fetches the linked pull request. The PR is found by searching for a branch whose name contains the Jira issue key — so `feature/CODE-123`, `fix/CODE-123`, and `docs/CODE-123` all link to `CODE-123` regardless of which branch you have checked out locally.

The **Pull Request** tab shows:

- PR title, status (Active / Completed / Abandoned), author, and source → target branches
- **Pipeline status** — latest Azure Pipelines run result (● In Progress / ✓ Succeeded / ✗ Failed / ○ Cancelled)
- **Reviewer votes** — each reviewer's current vote with color indicators:
  - ✓ green = Approved / Approved with suggestions
  - ✗ red = Rejected
  - ⏳ yellow = Waiting for author
  - ○ gray = No vote
- **Changed files** list with change type indicators (A added / M modified / D deleted / R renamed)
- **Unified diff** for each file, rendered lazygit-style with colored `+`/`-` lines and `@@` hunk headers

Switch between the **Jira** and **Pull Request** tabs with `[` and `]`.

### Voting on a Pull Request

Press `v` while on the Pull Request tab to open the voting modal:

| Option                   | Vote value |
|--------------------------|------------|
| Approve                  | ✓ Approved |
| Approve with suggestions | ✓ Approved with suggestions |
| Reset vote               | ○ No vote  |
| Wait for author          | ⏳ Waiting for author |
| Reject                   | ✗ Rejected |

Navigate with `j`/`k`, select with `Enter` or a number key, cancel with `Esc`. After voting, the reviewer list refreshes automatically to show your updated vote.

## Health Check

```
:checkhealth anvil
```

## ADF Support

Jira uses [Atlassian Document Format (ADF)](https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/) for rich text. anvil.nvim:

- **Renders** ADF descriptions, comments, and custom fields to readable plain text
- **Edits** via `ADF → Markdown → $EDITOR → Markdown → ADF` round-trip
- Supports: paragraphs, headings, lists, code blocks, blockquotes, tables, mentions, inline marks

## License

Apache 2.0
