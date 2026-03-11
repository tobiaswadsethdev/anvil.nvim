local M = {}

function M.check()
  local health = vim.health

  health.start("anvil.nvim")

  -- 1. Check snacks.nvim is available
  local ok_snacks = pcall(require, "snacks")
  if ok_snacks then
    health.ok("snacks.nvim is installed")
  else
    health.error(
      "snacks.nvim is not installed",
      { "Install snacks.nvim: https://github.com/folke/snacks.nvim" }
    )
  end

  -- 2. Check jira-anvil binary
  local config = require("anvil.config")
  local bin = config.bin()
  if bin then
    health.ok("jira-anvil binary found: " .. bin)
  else
    health.error(
      "jira-anvil binary not found on PATH",
      {
        "Option 1 – build from source:",
        "  cd <plugin_dir>/cmd/jira-anvil && go install .",
        "Option 2 – ensure ~/go/bin is in your PATH",
        "Option 3 – set config.bin_path in require('anvil').setup()",
      }
    )
  end

  -- 3. Check config file
  local config_path = vim.fn.expand("~/.config/anvil/config.yaml")
  if vim.fn.filereadable(config_path) == 1 then
    health.ok("Config file found: " .. config_path)
  else
    health.warn(
      "Config file not found: " .. config_path,
      {
        "Call require('anvil').setup({...}) to generate it automatically,",
        "or create it manually. See :help anvil-config",
      }
    )
  end

  -- 4. Validate config values
  local opts = config.options
  if opts.jira and opts.jira.url ~= "" then
    health.ok("Jira URL configured: " .. opts.jira.url)
  else
    health.warn("Jira URL not configured", {
      "Set jira.url in require('anvil').setup() or JIRA_URL env var",
    })
  end

  if opts.jira and opts.jira.user ~= "" then
    health.ok("Jira user configured")
  else
    health.warn("Jira user not configured", {
      "Set jira.user in require('anvil').setup() or JIRA_USER env var",
    })
  end

  local token = opts.jira and opts.jira.token
  if (token and token ~= "") or vim.env.JIRA_API_TOKEN then
    health.ok("Jira API token configured")
  else
    health.warn("Jira API token not configured", {
      "Set jira.token in require('anvil').setup() or JIRA_API_TOKEN env var",
    })
  end

  -- 5. Check filter count
  local filters = opts.filters or {}
  if #filters > 0 then
    health.ok(string.format("%d JQL filter(s) configured", #filters))
  else
    health.warn("No JQL filters configured", {
      "Add filters in require('anvil').setup({ filters = {...} })",
    })
  end
end

return M
