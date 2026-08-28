local M = {}

local defaults = {
  api_url = "http://127.0.0.1:7341",
  view = "float", -- "float" or "side"
  -- Review mutation is deliberately opt-in. Configure the standalone
  -- lens-review command and explicit event/Trace paths in your own setup.
  review = nil,
}

local config = vim.deepcopy(defaults)
local state = { buffer = nil, window = nil, source = nil, snapshot = nil, review_index = 0 }
local live_namespace = vim.api.nvim_create_namespace("aideck_lens_live")
local review_namespace = vim.api.nvim_create_namespace("aideck_lens_review")

local function notify(message, level)
  vim.notify("Inspector: " .. message, level or vim.log.levels.INFO)
end

local function is_valid_buffer()
  return state.buffer and vim.api.nvim_buf_is_valid(state.buffer)
end

local function is_valid_window()
  return state.window and vim.api.nvim_win_is_valid(state.window)
end

local function encode(value)
  if vim.json and vim.json.encode then
    return vim.json.encode(value)
  end
  return vim.fn.json_encode(value)
end

local function decode(value)
  if vim.json and vim.json.decode then
    return vim.json.decode(value)
  end
  return vim.fn.json_decode(value)
end

local function pretty(value)
  if value == nil then
    return "not available"
  end
  local ok, text = pcall(encode, value)
  return ok and text or tostring(value)
end

local function valid_api(url)
  if type(url) ~= "string" then
    return false
  end
  return url:match("^http://127%.0%.0%.1[:/]" ) ~= nil
    or url:match("^http://localhost[:/]" ) ~= nil
    or url:match("^http://%[::1%][:/]") ~= nil
end

local function lines_for(snapshot)
  local active = snapshot.active or {}
  local context = snapshot.context or {}
  local health = snapshot.health or {}
  local output = {
    "Lens (read-only)",
    "",
    "Active scope",
    "  Directory: " .. (active.directory or "—"),
    "  Global: " .. (active.global or "—"),
    "  Workspace: " .. (active.workspace or "—"),
    "  Project: " .. (active.project or "—"),
    "  Sources: " .. table.concat(active.sources or {}, ", "),
    "",
    "Context",
    "  Mode: " .. (context.mode or "—"),
    "  Session: " .. (context.session_id or "—"),
    "  Manifest digest: " .. (context.manifest_digest or "—"),
    "  Rules: " .. tostring(context.rule_count or 0),
    "  Knowledge: " .. tostring(context.knowledge_count or 0),
    "  Code map: " .. (context.code_map_version or "—"),
    "",
    "Health: " .. (health.state or "unavailable"),
  }
  for _, service in ipairs(health.services or {}) do
    table.insert(output, string.format("  %s: %s%s", service.name or "source", service.state or "unknown", service.message and (" — " .. service.message) or ""))
  end
  local sections = {
    { "Lexicon", snapshot.lexicon },
    { "Scout proposals", snapshot.scout },
    { "Splice status", snapshot.splice or snapshot.pith },
    { "Receipts / test evidence", snapshot.receipts },
    { "Legacy migration status", snapshot.legacy_projx },
  }
  for _, section in ipairs(sections) do
    table.insert(output, "")
    table.insert(output, section[1])
    for line in pretty(section[2]):gmatch("[^\n]+") do
      table.insert(output, "  " .. line)
    end
  end

  for _, thread in ipairs(snapshot.threads or {}) do
    table.insert(output, "")
    table.insert(output, "Thread: " .. ((thread.entry or {}).name or "structural path"))
    for _, node in ipairs(thread.nodes or {}) do
      local symbol = node.symbol or {}
      local links = ""
      if #(node.callers or {}) > 0 then links = links .. " callers:" .. tostring(#node.callers) end
      if #(node.calls or {}) > 0 then links = links .. " calls:" .. tostring(#node.calls) end
      table.insert(output, string.format("  %s  %s:%s%s", symbol.name or symbol.id or "node", symbol.file or "?", symbol.line or "?", links))
    end
    if #(thread.nodes or {}) == 0 then table.insert(output, "  No structural nodes") end
    if thread.truncated then table.insert(output, "  Bounded path truncated; request a larger Atlas Thread explicitly.") end
  end

  if snapshot.reconciliation then
    local r = snapshot.reconciliation
    table.insert(output, "")
    table.insert(output, "Intent reconciliation: " .. (r.state or "unknown"))
    if r.goal then table.insert(output, "  Goal: " .. r.goal) end
    for _, value in ipairs(r.unexpected_files or {}) do table.insert(output, "  Unexpected file: " .. value) end
    for _, value in ipairs(r.missing_symbols or {}) do table.insert(output, "  Missing symbol: " .. value) end
    for _, value in ipairs(r.missing_validations or {}) do table.insert(output, "  Missing validation: " .. value) end
    for _, value in ipairs(r.failed_validations or {}) do table.insert(output, "  Failed validation: " .. value) end
  end

  local live = snapshot.live
  if live then
    table.insert(output, "")
    table.insert(output, "AI Live")
    table.insert(output, "  Goal: " .. (live.goal or "â€”"))
    if live.current then
      table.insert(output, "  Now: " .. (live.current.symbol or live.current.file or live.current.kind or "â€”"))
      if live.current.why then table.insert(output, "  Why: " .. live.current.why) end
    else
      table.insert(output, "  Now: idle")
    end
    table.insert(output, "")
    table.insert(output, "Review queue")
    for _, item in ipairs(live.review_queue or {}) do
      table.insert(output, string.format("  %s  %s%s", item.state or "NEW", item.symbol or item.file or item.id,
        item.why and (" â€” " .. item.why) or ""))
    end
    if #(live.review_queue or {}) == 0 then table.insert(output, "  No unreviewed changes") end
    if live.scope then
      table.insert(output, "")
      table.insert(output, "Scope: " .. (live.scope.state or "unknown"))
      table.insert(output, "  Files: " .. tostring(#(live.scope.observed_files or {})) .. (live.scope.expected_files_max and live.scope.expected_files_max > 0 and (" / budget " .. tostring(live.scope.expected_files_max)) or ""))
      for _, file in ipairs(live.scope.unexpected_files or {}) do table.insert(output, "  Unexpected: " .. file) end
    end
    table.insert(output, "")
    table.insert(output, "Validation")
    for _, check in ipairs(live.validation or {}) do
      table.insert(output, string.format("  %s  %s", check.state or "CHECKING", check.kind or check.target or check.id))
    end
    if #(live.validation or {}) == 0 then table.insert(output, "  No validation events") end
  end
  return output
end

local function absolute_location(file)
  if type(file) ~= "string" or file == "" then return nil end
  if file:match("^%a:[/\\]") or file:sub(1, 1) == "/" then return file end
  local active = (state.snapshot or {}).active or {}
  if type(active.directory) ~= "string" or active.directory == "" then return nil end
  return active.directory .. "/" .. file
end

local function target_window()
  for _, window in ipairs(vim.api.nvim_tabpage_list_wins(0)) do
    if window ~= state.window and vim.api.nvim_win_is_valid(window) then return window end
  end
  if is_valid_window() then return nil end
  return vim.api.nvim_get_current_win()
end

local function mark_location(buffer, location, label)
  vim.api.nvim_buf_clear_namespace(buffer, live_namespace, 0, -1)
  local line = math.max(0, (tonumber(location.start_line) or 1) - 1)
  vim.api.nvim_buf_set_extmark(buffer, live_namespace, line, 0, {
    virt_text = { { label, "DiagnosticInfo" } },
    virt_text_pos = "eol",
  })
end

local function review_mark(item)
  local labels = { NEW = "A+", CHECKING = "A~", FAILED = "A!", PASSING = "A✓", REVIEWED = "U✓", ACCEPTED = "OK", REJECTED = "X " }
  local groups = { FAILED = "DiagnosticError", REJECTED = "DiagnosticError", PASSING = "DiagnosticOk", ACCEPTED = "DiagnosticOk", NEW = "DiagnosticInfo", CHECKING = "DiagnosticWarn", REVIEWED = "DiagnosticHint" }
  return labels[item.state or "NEW"] or "A?", groups[item.state or "NEW"] or "DiagnosticInfo"
end

local function decorate_reviews(buffer)
  vim.api.nvim_buf_clear_namespace(buffer, review_namespace, 0, -1)
  local queue = (((state.snapshot or {}).live or {}).review_queue) or {}
  local name = vim.api.nvim_buf_get_name(buffer):gsub("\\", "/")
  for _, item in ipairs(queue) do
    local path = absolute_location(item.file)
    if path and path:gsub("\\", "/") == name then
      local text, group = review_mark(item)
      vim.api.nvim_buf_set_extmark(buffer, review_namespace, math.max(0, (tonumber(item.start_line) or 1) - 1), 0, {
        sign_text = text, sign_hl_group = group,
        virt_text = { { " " .. text .. " " .. (item.symbol or item.file or "AI change"), group } },
        virt_text_pos = "eol",
      })
    end
  end
end

local function jump_to(location, label)
  local path = absolute_location(location and location.file)
  if not path or vim.fn.filereadable(path) ~= 1 then
    notify("source location is unavailable", vim.log.levels.WARN)
    return false
  end
  local destination = target_window()
  if not destination then
    notify("open a code window before following Lens activity", vim.log.levels.WARN)
    return false
  end
  vim.api.nvim_set_current_win(destination)
  vim.cmd("edit " .. vim.fn.fnameescape(path))
  local buffer = vim.api.nvim_get_current_buf()
  decorate_reviews(buffer)
  vim.api.nvim_win_set_cursor(destination, { math.max(1, tonumber(location.start_line) or 1), 0 })
  mark_location(buffer, location, label or "AI change")
  return true
end

local function render(lines)
  if not is_valid_buffer() then
    return
  end
  vim.bo[state.buffer].modifiable = true
  vim.api.nvim_buf_set_lines(state.buffer, 0, -1, false, lines)
  vim.bo[state.buffer].modifiable = false
  vim.bo[state.buffer].modified = false
end

local function render_error(message)
  render({
    "Lens (read-only)",
    "",
    "Inspector is currently unavailable.",
    message,
    "",
    "Start the separate Inspector API or pass --snapshot=/explicit/file.json.",
    "No source data was changed.",
  })
end

local function make_window(view)
  if is_valid_window() then
    vim.api.nvim_set_current_win(state.window)
    return
  end
  local buffer = vim.api.nvim_create_buf(false, true)
  state.buffer = buffer
  vim.bo[buffer].buftype = "nofile"
  vim.bo[buffer].bufhidden = "wipe"
  vim.bo[buffer].swapfile = false
  vim.bo[buffer].filetype = "inspector"
  vim.bo[buffer].modifiable = false
  if view == "side" then
    vim.cmd("botright vsplit")
    state.window = vim.api.nvim_get_current_win()
    vim.api.nvim_win_set_buf(state.window, buffer)
    vim.api.nvim_win_set_width(state.window, math.max(48, math.floor(vim.o.columns * 0.38)))
  else
    local width = math.max(60, math.floor(vim.o.columns * 0.82))
    local height = math.max(18, math.floor(vim.o.lines * 0.78))
    state.window = vim.api.nvim_open_win(buffer, true, {
      relative = "editor", style = "minimal", border = "rounded", title = " Inspector ", title_pos = "center",
      width = width, height = height,
      row = math.max(0, math.floor((vim.o.lines - height) / 2 - 1)),
      col = math.max(0, math.floor((vim.o.columns - width) / 2)),
    })
  end
  vim.keymap.set("n", "q", M.close, { buffer = buffer, silent = true, desc = "Close Inspector" })
  vim.keymap.set("n", "r", M.refresh, { buffer = buffer, silent = true, desc = "Refresh Inspector" })
  vim.keymap.set("n", "]u", function() M.review_next(1) end, { buffer = buffer, silent = true, desc = "Next AI review item" })
  vim.keymap.set("n", "[u", function() M.review_next(-1) end, { buffer = buffer, silent = true, desc = "Previous AI review item" })
  vim.keymap.set("n", "a", function() M.review_decide("accepted") end, { buffer = buffer, silent = true, desc = "Accept selected AI review" })
  vim.keymap.set("n", "x", function() M.review_decide("rejected") end, { buffer = buffer, silent = true, desc = "Reject selected AI review" })
  vim.keymap.set("n", "v", function() M.review_decide("reviewed") end, { buffer = buffer, silent = true, desc = "Mark selected AI review" })
end

local function read_snapshot_file(path)
  local ok, data = pcall(vim.fn.readfile, path)
  if not ok then
    return nil, "cannot read snapshot file: " .. path
  end
  local decoded, error = pcall(decode, table.concat(data, "\n"))
  if not decoded then
    return nil, "snapshot is not valid JSON: " .. tostring(error)
  end
  return error
end

local function read_api(url, callback)
  if not valid_api(url) then
    callback(nil, "only loopback Inspector API URLs are allowed")
    return
  end
  local target = url:gsub("/$", "") .. "/v1/snapshot"
  if vim.system then
    vim.system({ "curl", "-fsSL", target }, { text = true }, function(result)
      vim.schedule(function()
        if result.code ~= 0 then
          callback(nil, "cannot reach Inspector API at " .. target)
          return
        end
        local ok, snapshot = pcall(decode, result.stdout)
        callback(ok and snapshot or nil, ok and nil or "Inspector API returned invalid JSON")
      end)
    end)
    return
  end
  vim.fn.jobstart({ "curl", "-fsSL", target }, {
    stdout_buffered = true,
    on_stdout = function(_, data)
      local ok, snapshot = pcall(decode, table.concat(data or {}, "\n"))
      vim.schedule(function() callback(ok and snapshot or nil, ok and nil or "Inspector API returned invalid JSON") end)
    end,
    on_stderr = function() end,
    on_exit = function(_, code)
      if code ~= 0 then vim.schedule(function() callback(nil, "cannot reach Inspector API at " .. target) end) end
    end,
  })
end

function M.setup(options)
  config = vim.tbl_deep_extend("force", config, options or {})
end

function M.parse_args(args)
  local result = {}
  for _, value in ipairs(args) do
    local api = value:match("^%-%-api=(.+)$")
    local snapshot = value:match("^%-%-snapshot=(.+)$")
    if api then result.api_url = api
    elseif snapshot then result.snapshot_file = snapshot
    else return nil, "InspectorOpen accepts only --api=URL or --snapshot=PATH" end
  end
  if result.api_url and result.snapshot_file then
    return nil, "choose either --api or --snapshot, not both"
  end
  return result
end

function M.open(options)
  options = options or {}
  make_window(options.view or config.view)
  state.source = options.snapshot_file and { snapshot_file = options.snapshot_file } or { api_url = options.api_url or config.api_url }
  M.refresh()
end

-- live is a convenience entrypoint for the bounded event projection. The
-- caller still supplies a loopback Lens API or explicit snapshot; this plugin
-- never starts a watcher or steals ownership of the worker process.
function M.live(options)
  M.open(options)
end

function M.refresh()
  if not is_valid_buffer() then
    notify("open Inspector first", vim.log.levels.WARN)
    return
  end
  if state.source and state.source.snapshot_file then
    local snapshot, error = read_snapshot_file(state.source.snapshot_file)
    if snapshot then
      state.snapshot, state.review_index = snapshot, 0
      render(lines_for(snapshot))
    else render_error(error) end
    return
  end
  render({ "Inspector (read-only)", "", "Refreshing local Inspector API…" })
  read_api((state.source and state.source.api_url) or config.api_url, function(snapshot, error)
    if snapshot then
      state.snapshot, state.review_index = snapshot, 0
      render(lines_for(snapshot))
    else render_error(error) end
  end)
end

-- Follow is intentionally a user action, not a watcher. It moves to the
-- latest coherent edit when asked and never steals the viewport afterward.
function M.follow()
  local live = (state.snapshot or {}).live or {}
  if not live.current then
    notify("no current AI edit is available", vim.log.levels.WARN)
    return
  end
  jump_to(live.current, "AI editing " .. (live.current.symbol or live.current.file or "current change"))
end

function M.review_next(direction)
  local queue = (((state.snapshot or {}).live or {}).review_queue) or {}
  if #queue == 0 then
    notify("review queue is empty", vim.log.levels.INFO)
    return
  end
  state.review_index = ((state.review_index + (direction or 1) - 1) % #queue) + 1
  local item = queue[state.review_index]
  jump_to(item, "AI " .. (item.state or "NEW") .. " " .. (item.symbol or item.file or "change"))
end

local function decision_argv(item, decision)
  local review = config.review
  if type(review) ~= "table" or type(review.command) ~= "table" or #review.command == 0 or type(review.events_file) ~= "string" or review.events_file == "" then
    return nil, "configure review.command and review.events_file to make a durable review decision"
  end
  local argv = vim.deepcopy(review.command)
  vim.list_extend(argv, { "--events", review.events_file, "--review-id", item.id, "--decision", decision, "--actor", review.actor or "human", "--scope", review.scope or "project" })
  if type(review.trace) == "table" and review.trace.binary and review.trace.app and review.trace.storage_root then
    vim.list_extend(argv, { "--trace", review.trace.binary, "--trace-app", review.trace.app, "--trace-storage-root", review.trace.storage_root })
    if review.trace.instance then vim.list_extend(argv, { "--trace-instance", review.trace.instance }) end
  end
  return argv
end

function M.review_decide(decision)
  local queue = (((state.snapshot or {}).live or {}).review_queue) or {}
  local item = queue[state.review_index]
  if not item then
    notify("select a review item first with LensReviewNext", vim.log.levels.WARN)
    return
  end
  local argv, err = decision_argv(item, decision)
  if not argv then notify(err, vim.log.levels.WARN); return end
  if not vim.system then notify("Lens review decisions require Neovim 0.10+", vim.log.levels.ERROR); return end
  vim.system(argv, { text = true }, function(result)
    vim.schedule(function()
      if result.code ~= 0 then
        notify("review decision failed: " .. (result.stderr or "unknown error"), vim.log.levels.ERROR)
        return
      end
      item.state = decision == "accepted" and "ACCEPTED" or (decision == "rejected" and "REJECTED" or "REVIEWED")
      render(lines_for(state.snapshot))
      local current = vim.api.nvim_get_current_buf()
      if current and vim.api.nvim_buf_is_valid(current) then decorate_reviews(current) end
      notify("review " .. item.state:lower() .. " and persisted")
    end)
  end)
end

-- Exposed for headless contract tests; not a separate public protocol.
M._review_argv = decision_argv

function M.close()
  if is_valid_window() then vim.api.nvim_win_close(state.window, true) end
  state.window, state.buffer = nil, nil
end

return M
