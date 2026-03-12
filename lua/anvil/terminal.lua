--- anvil.nvim terminal — lightweight terminal window manager.
--- Manages terminal windows natively via the Neovim API.
--- Supports floating, right-split, and bottom-split layouts.
local M = {}

-- Per-command state: _terms[key] = { buf = bufnr, win = winnr|nil }
local _terms = {}

--- Build the window config for a split or floating layout.
---@param buf    integer  buffer to display
---@param opts   table    position, rounded, width, height
---@return integer  window handle (now current window)
local function open_window(buf, opts)
  local position = opts.position or "float"

  if position == "right" then
    local width = opts.width or math.floor(vim.o.columns * 0.4)
    vim.cmd("botright vsplit")
    local win = vim.api.nvim_get_current_win()
    vim.api.nvim_win_set_buf(win, buf)
    vim.api.nvim_win_set_width(win, width)
    return win

  elseif position == "bottom" then
    local height = opts.height or math.floor(vim.o.lines * 0.3)
    vim.cmd("botright split")
    local win = vim.api.nvim_get_current_win()
    vim.api.nvim_win_set_buf(win, buf)
    vim.api.nvim_win_set_height(win, height)
    return win

  else -- float (default)
    local width  = opts.width  or math.floor(vim.o.columns * 0.9)
    local height = opts.height or math.floor(vim.o.lines  * 0.85)
    local row    = math.floor((vim.o.lines   - height) / 2)
    local col    = math.floor((vim.o.columns - width)  / 2)
    local border = opts.rounded and "rounded" or "none"
    return vim.api.nvim_open_win(buf, true, {
      relative = "editor",
      row      = row,
      col      = col,
      width    = width,
      height   = height,
      style    = "minimal",
      border   = border,
      zindex   = 50,
    })
  end
end

--- open opens a terminal window running cmd.
---
--- Options:
---   position    "float" | "right" | "bottom"  (default: "float")
---   rounded     boolean  rounded border on floating windows  (default: false)
---   width       integer  explicit width in columns           (optional)
---   height      integer  explicit height in rows             (optional)
---   cwd         string   working directory for the process   (optional)
---   auto_insert boolean  enter insert mode automatically     (default: true)
---   auto_close  boolean  wipe buffer when process exits      (default: true)
---
---@param cmd    string[]  command and arguments
---@param opts?  table
---@return integer|nil  window handle
function M.open(cmd, opts)
  opts = opts or {}
  local key = table.concat(cmd, " ")
  local st  = _terms[key]

  -- Window already open: just focus it
  if st and vim.api.nvim_win_is_valid(st.win or -1) then
    vim.api.nvim_set_current_win(st.win)
    if opts.auto_insert ~= false then vim.cmd("startinsert") end
    return st.win
  end

  -- Reuse an existing terminal buffer or create a fresh one
  local buf, is_new
  if st and vim.api.nvim_buf_is_valid(st.buf) then
    buf, is_new = st.buf, false
  else
    buf    = vim.api.nvim_create_buf(false, true)
    is_new = true
  end

  -- Keep the buffer alive when its window is closed (needed for toggle)
  vim.bo[buf].bufhidden = "hide"

  local win = open_window(buf, opts)
  _terms[key] = { buf = buf, win = win }

  if is_new then
    vim.fn.termopen(cmd, { cwd = opts.cwd })

    if opts.auto_close ~= false then
      vim.api.nvim_create_autocmd("TermClose", {
        buffer   = buf,
        once     = true,
        callback = function()
          vim.schedule(function()
            local st_now = _terms[key]
            _terms[key] = nil
            -- Close the window if it is still open
            if st_now and vim.api.nvim_win_is_valid(st_now.win or -1) then
              vim.api.nvim_win_close(st_now.win, true)
            end
            if vim.api.nvim_buf_is_valid(buf) then
              vim.api.nvim_buf_delete(buf, { force = true })
            end
          end)
        end,
      })
    end
  end

  if opts.auto_insert ~= false then
    vim.cmd("startinsert")
  end

  return win
end

--- toggle hides the terminal window if it is open, or opens/re-opens it if hidden.
---@param cmd    string[]  command and arguments
---@param opts?  table     same options as open()
function M.toggle(cmd, opts)
  opts = opts or {}
  local key = table.concat(cmd, " ")
  local st  = _terms[key]

  if st and vim.api.nvim_win_is_valid(st.win or -1) then
    -- Close the window; buffer survives because bufhidden = "hide"
    vim.api.nvim_win_close(st.win, true)
    _terms[key] = { buf = st.buf, win = nil }
    return
  end

  M.open(cmd, opts)
end

return M
