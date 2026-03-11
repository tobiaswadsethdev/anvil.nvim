-- anvil.nvim plugin loader
-- Registers user commands. Setup must be called explicitly by the user.

if vim.g.loaded_anvil then
  return
end
vim.g.loaded_anvil = true

vim.api.nvim_create_user_command("Anvil", function()
  require("anvil").open()
end, { desc = "Open Anvil Jira TUI" })

vim.api.nvim_create_user_command("AnvilToggle", function()
  require("anvil").toggle()
end, { desc = "Toggle Anvil Jira TUI" })
