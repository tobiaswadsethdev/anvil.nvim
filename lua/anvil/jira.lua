local M = {}

local config   = require("anvil.config")
local terminal = require("anvil.terminal")

--- open opens the jira-anvil TUI.
---@param opts? table  extra window opts (overrides config.win)
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

  -- Regenerate config YAML on every open so Neovim-side changes take effect
  config.write_yaml()

  local term_opts = vim.tbl_deep_extend("force", {
    cwd         = vim.fn.getcwd(),
    auto_insert = true,
    auto_close  = true,
  }, config.options.win or {}, opts or {})

  return terminal.open({ bin }, term_opts)
end

--- toggle toggles the jira-anvil TUI.
---@param opts? table  extra window opts (overrides config.win)
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

  return terminal.toggle({ bin }, term_opts)
end

return M
