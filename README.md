# anvil.nvim

A Jira Cloud TUI for Neovim. Browse issues, manage transitions, review pull requests, and write comments without leaving your editor — powered by a `jira-anvil` Go binary launched in a built-in terminal window.

## Features

- **JQL filter cycling** — define named filters and cycle through them with `[` / `]`
- **Issue list** — table view with key, summary, status, priority, assignee, and age
- **Issue detail** — scrollable view with ADF description rendering, comments, and custom fields
- **Transitions** — change issue status via modal
- **Comments** — add comments (Markdown → ADF)
- **Assign** — fuzzy-search and assign users
- **Edit** — edit description and ADF custom fields in `$EDITOR`
- **Browser** — open any issue in the browser with `o`
- **Azure DevOps PR tab** — linked pull request with git diff and pipeline status, shown as a tab alongside the Jira issue
- **PR voting** — view reviewer votes and cast your own (Approve / Reject / etc.) directly from the PR tab

## Requirements

- Neovim ≥ 0.9
- Go ≥ 1.21 (to build `jira-anvil`)
- Jira Cloud account + API token
- Azure DevOps Personal Access Token *(optional, for PR tab)*

## Installation

**lazy.nvim** (the `build` key handles compiling the binary automatically):

```lua
{
  "tobiaswadsethdev/anvil.nvim",
  build = "make install",
  opts = {
    jira = {
      url   = "https://yourcompany.atlassian.net",
      user  = "you@example.com",
      token = vim.env.JIRA_API_TOKEN,  -- keep secrets in env, not config
    },
    filters = {
      { name = "My Issues", jql = "assignee = currentUser() AND status != Done ORDER BY updated DESC" },
      { name = "My Sprint", jql = "project = PROJ AND sprint in openSprints() ORDER BY priority ASC" },
    },
  },
}
```

<details>
<summary>Manual binary build</summary>

```sh
# From the plugin directory
make install

# Or via go install
cd cmd/jira-anvil && go install .
```

Ensure `~/go/bin` is in your `PATH`.
</details>

## Configuration

Full options table (all fields except `jira` are optional):

```lua
require('anvil').setup({
  -- Jira Cloud connection (required)
  jira = {
    url   = "https://yourcompany.atlassian.net",
    user  = "you@example.com",
    token = vim.env.JIRA_API_TOKEN,
  },

  -- Azure DevOps integration — enables the PR tab in issue detail
  azdo = {
    url     = "https://dev.azure.com/myorg",
    project = "myproject",
    repo    = "myrepo",
    token   = vim.env.AZDO_TOKEN,
  },

  -- Named JQL filters (cycle with [ / ] in the TUI)
  filters = {
    { name = "My Issues",   jql = "assignee = currentUser() AND status != Done ORDER BY updated DESC" },
    { name = "Sprint",      jql = "project = PROJ AND sprint in openSprints() ORDER BY priority ASC" },
    { name = "Unassigned",  jql = "project = PROJ AND assignee is EMPTY AND status = 'To Do'" },
  },

  -- Keymaps to open / toggle the TUI (set to "" to disable)
  keymaps = {
    open   = "<leader>jj",
    toggle = "<leader>jt",
  },

  -- Explicit path to the jira-anvil binary (optional; defaults to PATH search)
  bin_path = nil,

  -- Terminal window options
  win = {
    position = "float",   -- "float" | "right" | "bottom"
    rounded  = false,     -- rounded border (floating windows only)
    -- width  = nil,      -- explicit width in columns  (optional)
    -- height = nil,      -- explicit height in rows    (optional)
  },
})
```

### Window layouts

| `position` | Description                                      |
|------------|--------------------------------------------------|
| `"float"`  | Centred floating window (default, 90 × 85 %)     |
| `"right"`  | Vertical split on the right (40 % of columns)    |
| `"bottom"` | Horizontal split at the bottom (30 % of lines)   |

When `position = "float"`, setting `rounded = true` draws a rounded border around the window.

**Environment variables** (take precedence over config values):

| Variable       | Description                      |
|----------------|----------------------------------|
| `JIRA_URL`     | Jira base URL                    |
| `JIRA_USER`    | Jira email address               |
| `JIRA_TOKEN`   | Jira API token                   |
| `AZDO_URL`     | Azure DevOps organization URL    |
| `AZDO_PROJECT` | Azure DevOps project name        |
| `AZDO_REPO`    | Git repository name              |
| `AZDO_TOKEN`   | Azure DevOps personal access token |

## Usage

| Key / Command        | Action              |
|----------------------|---------------------|
| `<leader>jj`         | Open Jira TUI       |
| `<leader>jt`         | Toggle Jira TUI     |
| `:Anvil`             | Open Jira TUI       |
| `:AnvilToggle`       | Toggle Jira TUI     |
| `:checkhealth anvil` | Verify setup        |

## TUI Keybindings

### Issue List

| Key         | Action                   |
|-------------|--------------------------|
| `[`         | Previous filter          |
| `]`         | Next filter              |
| `↑` / `k`   | Move up                  |
| `↓` / `j`   | Move down                |
| `Enter`     | Open issue detail        |
| `r`         | Refresh                  |
| `o`         | Open in browser          |
| `?`         | Show help                |
| `q`         | Quit                     |

### Issue Detail

| Key         | Action                           |
|-------------|----------------------------------|
| `[`         | Previous tab (Jira / PR)         |
| `]`         | Next tab (Jira / PR)             |
| `↑` / `k`   | Scroll up                        |
| `↓` / `j`   | Scroll down                      |
| `t`         | Transition status                |
| `c`         | Add comment                      |
| `a`         | Assign issue                     |
| `e`         | Edit in `$EDITOR`                |
| `v`         | Vote on PR *(PR tab only)*       |
| `o`         | Open in browser                  |
| `r`         | Reload                           |
| `?`         | Show help                        |
| `q` / `Esc` | Back to list                     |

## Azure DevOps PR Tab

When Azure DevOps is configured, opening any issue detail automatically fetches the linked pull request. The PR is found by searching for a branch whose name contains the Jira issue key — `feature/CODE-123`, `fix/CODE-123`, and `docs/CODE-123` all resolve to `CODE-123` regardless of your current local branch.

The **Pull Request** tab shows:

- PR title, status (Active / Completed / Abandoned), author, and source → target branches
- **Pipeline status** — latest Azure Pipelines run: `●` In Progress / `✓` Succeeded / `✗` Failed / `○` Cancelled
- **Reviewer votes** with color indicators:
  - `✓` green — Approved / Approved with suggestions
  - `✗` red — Rejected
  - `⏳` yellow — Waiting for author
  - `○` gray — No vote
- **Changed files** list with change type indicators (A / M / D / R)
- **Unified diff** with colored `+`/`-` lines and `@@` hunk headers

Switch between **Jira** and **Pull Request** tabs with `[` and `]`.

### Voting

Press `v` on the Pull Request tab to open the voting modal:

| Option                    | Vote              |
|---------------------------|-------------------|
| Approve                   | ✓ Approved        |
| Approve with suggestions  | ✓ Approved with suggestions |
| Reset vote                | ○ No vote         |
| Wait for author           | ⏳ Waiting for author |
| Reject                    | ✗ Rejected        |

Navigate with `j` / `k`, select with `Enter` or a number key, cancel with `Esc`. The reviewer list refreshes automatically after voting.

## ADF Support

Jira uses [Atlassian Document Format (ADF)](https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/) for rich text. anvil.nvim:

- **Renders** ADF descriptions, comments, and custom fields to readable plain text
- **Edits** via `ADF → Markdown → $EDITOR → Markdown → ADF` round-trip
- Supports paragraphs, headings, lists, code blocks, blockquotes, tables, mentions, and inline marks

## Health Check

Run `:checkhealth anvil` to verify that the `jira-anvil` binary is on your `PATH`, your Jira credentials are configured, and the window position option is valid.

## License

Apache 2.0
