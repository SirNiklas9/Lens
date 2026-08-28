if vim.g.loaded_inspector_nvim_client then
  return
end
vim.g.loaded_inspector_nvim_client = true

local inspector = require("inspector")

local function open(command)
  local opts, err = inspector.parse_args(command.fargs)
  if err then
    vim.notify(err, vim.log.levels.ERROR)
    return
  end
  inspector.open(opts)
end

local command_options = {
  nargs = "*",
  complete = function()
    return { "--api=", "--snapshot=" }
  end,
}

vim.api.nvim_create_user_command("LensOpen", open, vim.tbl_extend("force", command_options, {
  desc = "Open the read-only Lens view",
}))
vim.api.nvim_create_user_command("InspectorOpen", open, vim.tbl_extend("force", command_options, {
  desc = "Compatibility alias for LensOpen",
}))

vim.api.nvim_create_user_command("LensLive", function(command)
  local parsed, err = inspector.parse_args(command.fargs)
  if not parsed then
    vim.notify("Lens: " .. err, vim.log.levels.ERROR)
    return
  end
  inspector.live(parsed)
end, vim.tbl_extend("force", command_options, {
  desc = "Open Lens live review projection",
}))

vim.api.nvim_create_user_command("LensFollow", function()
  inspector.follow()
end, { desc = "Jump to the current coherent AI edit" })

vim.api.nvim_create_user_command("LensReviewNext", function()
  inspector.review_next(1)
end, { desc = "Jump to next AI review item" })

vim.api.nvim_create_user_command("LensReviewPrevious", function()
  inspector.review_next(-1)
end, { desc = "Jump to previous AI review item" })

vim.api.nvim_create_user_command("LensReviewAccept", function()
  inspector.review_decide("accepted")
end, { desc = "Durably accept selected AI review item" })

vim.api.nvim_create_user_command("LensReviewReject", function()
  inspector.review_decide("rejected")
end, { desc = "Durably reject selected AI review item" })

vim.api.nvim_create_user_command("LensReviewMark", function()
  inspector.review_decide("reviewed")
end, { desc = "Durably mark selected AI review item reviewed" })

vim.api.nvim_create_user_command("InspectorRefresh", function()
  inspector.refresh()
end, { desc = "Refresh the Inspector view" })

vim.api.nvim_create_user_command("InspectorClose", function()
  inspector.close()
end, { desc = "Close the Inspector view" })

vim.api.nvim_create_user_command("LensRefresh", function()
  inspector.refresh()
end, { desc = "Refresh the Lens view" })

vim.api.nvim_create_user_command("LensClose", function()
  inspector.close()
end, { desc = "Close the Lens view" })
