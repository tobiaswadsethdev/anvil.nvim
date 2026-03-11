local M = {}

local config = require("anvil.config")

--- open opens the jira-anvil TUI in a snacks.nvim floating terminal.
---@param opts? table  extra opts forwarded to Snacks.terminal
function M.open(opts)
  local bin = config.bin()
  if not bin then
    vim.notify(
      "anvil: jira-anvil binary not found.\n"
        .. "Run: cd " .. vim.fn.fnamemodify(debug.getinfo(1, "S").source:sub(2), ":h:h:h")
        .. "/cmd/jira-anvil && go install .",
      vim.log.levels.ERROR
    )
    return
  end

  -- Regenerate config YAML on every open (like lazygit regenerates its theme)
  config.write_yaml()

  local term_opts = vim.tbl_deep_extend("force", {
    cwd         = vim.fn.getcwd(),
    auto_insert = true,
    auto_close  = true,
  }, config.options.win or {}, opts or {})

  return Snacks.terminal.open({ bin }, term_opts)
end

--- toggle toggles the jira-anvil TUI.
---@param opts? table  extra opts forwarded to Snacks.terminal
function M.toggle(opts)
  local bin = config.bin()
  if not bin then
    vim.notify("anvil: jira-anvil binary not found.", vim.log.levels.ERROR)
    return
  end

  config.write_yaml()

  local term_opts = vim.tbl_deep_extend("force", {
    cwd         = vim.fn.getcwd(),
    auto_insert = true,
    auto_close  = true,
  }, config.options.win or {}, opts or {})

  return Snacks.terminal.toggle({ bin }, term_opts)
end

return M
