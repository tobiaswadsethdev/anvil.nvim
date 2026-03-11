--- anvil.nvim — Jira TUI for Neovim
--- Integrates the jira-anvil TUI via snacks.nvim, similar to how lazygit works.
---
--- Quick start:
---   require('anvil').setup({
---     jira = {
---       url   = "https://yourcompany.atlassian.net",
---       user  = "you@example.com",
---       token = vim.env.JIRA_API_TOKEN,
---     },
---     filters = {
---       { name = "My Issues", jql = "assignee = currentUser() ORDER BY updated DESC" },
---     },
---     keymaps = {
---       open   = "<leader>jj",
---       toggle = "<leader>jt",
---     },
---   })

local M = {}

---@class AnvilJiraOpts
---@field url?   string  Jira Cloud base URL
---@field user?  string  Jira user email
---@field token? string  Jira API token

---@class AnvilFilterOpts
---@field name string  Display name for the filter
---@field jql  string  JQL query string

---@class AnvilKeymapOpts
---@field open?   string  Keymap to open the TUI
---@field toggle? string  Keymap to toggle the TUI

---@class AnvilOpts
---@field jira?     AnvilJiraOpts
---@field filters?  AnvilFilterOpts[]
---@field keymaps?  AnvilKeymapOpts
---@field bin_path? string  Explicit path to the jira-anvil binary
---@field win?      table   Extra opts forwarded to Snacks.terminal

--- setup configures anvil.nvim.
---@param opts? AnvilOpts
function M.setup(opts)
  local config = require("anvil.config")
  config.setup(opts or {})

  -- Register keymaps
  local keymaps = config.options.keymaps or {}
  if keymaps.open and keymaps.open ~= "" then
    vim.keymap.set("n", keymaps.open, function()
      M.open()
    end, { desc = "Anvil: Open Jira TUI", silent = true })
  end

  if keymaps.toggle and keymaps.toggle ~= "" then
    vim.keymap.set("n", keymaps.toggle, function()
      M.toggle()
    end, { desc = "Anvil: Toggle Jira TUI", silent = true })
  end
end

--- open opens the Jira TUI in a snacks.nvim floating terminal.
---@param opts? table  extra opts forwarded to Snacks.terminal
function M.open(opts)
  return require("anvil.jira").open(opts)
end

--- toggle toggles the Jira TUI.
---@param opts? table  extra opts forwarded to Snacks.terminal
function M.toggle(opts)
  return require("anvil.jira").toggle(opts)
end

return M
