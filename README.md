# anvil.nvim

A Jira Cloud TUI for Neovim. Browse issues, manage transitions, review pull requests, and write comments without leaving your editor — powered by a `jira-anvil` Go binary launched in a built-in terminal window.

## Features

- **JQL filter cycling** — define named filters and cycle through them with `[` / `]`
- **Issue list** — table view with key, summary, status, priority, assignee, and age
- **Issue detail** — lazygit-style multi-panel view with a responsive grid: issue metadata, PR overview, changes, and discussion/history
- **Create issue** — multi-step flow: project picker → issue type picker → YAML template in `$EDITOR`
- **Transitions** — change issue status via modal
- **Comments** — add comments (Markdown → ADF)
- **Assign** — fuzzy-search and assign users
- **Edit** — edit description and ADF custom fields in `$EDITOR`
- **Browser** — open any issue in the browser with `o`
- **Azure DevOps PR integration** — linked pull request data shown in dedicated overview/changes/discussion panels with Jira context tabs
- **PR voting** — view reviewer votes and cast your own (Approve / Reject / etc.) via `v`
- **PR comments** — view existing PR comment threads and add new ones or reply to threads via `c`

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
    repo    = "myrepo",   -- optional: if omitted, all repos in the project are searched
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
| `AZDO_REPO`    | Git repository name *(optional)* |
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
| `n`         | Create new issue         |
| `r`         | Refresh                  |
| `o`         | Open in browser          |
| `?`         | Show help                |
| `q`         | Quit                     |

### Issue Detail

The detail view uses a **lazygit-style multi-panel layout**. Without Azure DevOps configured there are 2 panels; with it there are 4 panels arranged in a 3-column grid:

| Panel | Label | Contents |
|-------|-------|----------|
| `[1]` | Issue Info | Key, status, priority, assignee, reporter, dates, labels |
| `[2]` | Pull Request | PR status, branches, pipeline, reviewers *(AzDO only)* |
| `[3]` | Changes | PR changed files/diff and Jira description (tabs: `Files` \| `Diff` \| `Jira Description`) *(AzDO only)* |
| `[4]` | Discussion | PR comments, Jira comments, and Jira history (tabs: `PR Comments` \| `Jira Comments` \| `Jira History`) *(AzDO only)* |

| Key             | Action                                            |
|-----------------|---------------------------------------------------|
| `Tab`           | Focus next panel                                  |
| `Shift+Tab`     | Focus previous panel                              |
| `1` – `4`       | Jump directly to panel by number                  |
| `[`             | Previous tab within focused panel                 |
| `]`             | Next tab within focused panel                     |
| `↑` / `k`       | Scroll up in focused panel                        |
| `↓` / `j`       | Scroll down in focused panel                      |
| `t`             | Transition status                                 |
| `c`             | Add Jira comment / Add PR comment *(PR panels)*   |
| `a`             | Assign issue                                      |
| `e`             | Edit in `$EDITOR`                                 |
| `v`             | Vote on PR *(requires AzDO)*                      |
| `y`             | Copy PR link to clipboard *(requires AzDO)*       |
| `o`             | Open in browser                                   |
| `r`             | Reload                                            |
| `?`             | Show help                                         |
| `q` / `Esc`     | Back to list                                      |

## Azure DevOps PR Panels

When Azure DevOps is configured, opening any issue detail automatically fetches the linked pull request. The PR is found by searching for a branch whose name contains the Jira issue key — `feature/CODE-123`, `fix/CODE-123`, and `docs/CODE-123` all resolve to `CODE-123` regardless of your current local branch.

If `repo` is not set, **all repositories** in the configured project are searched in order and the first matching PR is used. Set `repo` to restrict the search to a single repository and speed up lookup.

### Personal Access Token scopes

Create your PAT at `https://dev.azure.com/<org>/_usersSettings/tokens` with at minimum:

| Scope | Permission | Required for                              |
|-------|------------|-------------------------------------------|
| Code  | Read       | Viewing PRs, diffs, and file contents     |
| Code  | Write      | Voting on PRs and adding comments         |
| Build | Read       | Displaying pipeline build status          |

PR and Jira collaboration data is displayed across three dedicated areas in the detail view:

**Panel `[2]` — Pull Request** (left column, compact overview):
- PR status (Active / Completed / Abandoned), author, and source → target branches
- **Pipeline status** — latest Azure Pipelines run: `●` In Progress / `✓` Succeeded / `✗` Failed / `○` Cancelled
- **Reviewer votes** with color indicators:
  - `✓` green — Approved / Approved with suggestions
  - `✗` red — Rejected
  - `⏳` yellow — Waiting for author
  - `○` gray — No vote

**Panel `[3]` — Changes** (middle column, scrollable, 3 tabs):
- **Files tab** — changed files list with change type indicators (A / M / D / R)
- **Diff tab** — unified diff with colored `+`/`-` lines and `@@` hunk headers
- **Jira Description tab** — Jira issue description rendered from ADF

**Panel `[4]` — Discussion** (right column, scrollable, 3 tabs):
- **PR Comments tab** — PR comment threads numbered `[1]`, `[2]`, ... with author, timestamp, and replies
- **Jira Comments tab** — Jira issue comments (author + relative timestamp)
- **Jira History tab** — compact changelog entries (`time • author • field: from -> to`) with key fields prioritized

Navigate to a panel with its number key or `Tab`/`Shift+Tab`. Switch between tabs within focused tabbed panels (`[3]` and `[4]`) with `[` and `]`.

### Voting

Press `v` (from any panel) to open the voting modal:

| Option                    | Vote              |
|---------------------------|-------------------|
| Approve                   | ✓ Approved        |
| Approve with suggestions  | ✓ Approved with suggestions |
| Reset vote                | ○ No vote         |
| Wait for author           | ⏳ Waiting for author |
| Reject                    | ✗ Rejected        |

Navigate with `j` / `k`, select with `Enter` or a number key, cancel with `Esc`. The reviewer list refreshes automatically after voting.

### PR Comments

Press `c` while focused on a PR panel (`[2]`, `[3]`, or `[4]` when the PR Comments tab is active) to open the PR comment modal. It guides you through a multi-step flow:

**Step 1 — Choose comment type:**

| # | Type | Description |
|---|------|-------------|
| 1 | General comment | A top-level PR comment with no file context |
| 2 | File comment | A comment anchored to a specific file |
| 3 | Code comment | A comment anchored to a file and line number |
| 4 | Reply to thread | A reply to an existing comment thread |

**Step 2 (File / Code only)** — Enter the file path. Changed files from the PR diff are shown as hints below the input.

**Step 3 (Code only)** — Enter the line number.

**Step 3 (Reply only)** — Select the thread to reply to from a numbered list.

**Final step** — Write your comment text. Press `Ctrl+S` to submit or `Esc` to go back.

After submission, the comments section refreshes automatically. Use `Esc` at any step to navigate back through the flow.

## Create Issue

Press `n` from the issue list to create a new Jira issue. The flow is multi-step:

1. **Project picker** — fuzzy-search and select the target Jira project.
2. **Issue type picker** — select the issue type (Bug, Story, Task, etc.) available for that project.
3. **YAML editor** — a YAML template is generated and opened in `$EDITOR`. Required fields are marked `[REQUIRED]`. Fill in the values, save, and close to submit.

Field type hints and allowed values are shown as comments in the template. Markdown is supported for description and other rich-text fields (converted to ADF on submit).

## ADF Support

Jira uses [Atlassian Document Format (ADF)](https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/) for rich text. anvil.nvim:

- **Renders** ADF descriptions, comments, and custom fields to readable plain text
- **Edits** via `ADF → Markdown → $EDITOR → Markdown → ADF` round-trip
- Supports paragraphs, headings, lists, code blocks, blockquotes, tables, mentions, and inline marks

## Health Check

Run `:checkhealth anvil` to verify that the `jira-anvil` binary is on your `PATH`, your Jira credentials are configured, and the window position option is valid.

## License

Apache 2.0
