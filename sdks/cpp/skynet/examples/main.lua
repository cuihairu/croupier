-- Main service for Croupier SDK Skynet example
-- Copyright 2025 Croupier Authors

local skynet = require "skynet"
local croupier_service = ".croupier"

-- Configuration
local CROUPIER_SERVER = skynet.getenv("croupier_server") or "localhost:50051"
local AUTH_TOKEN = skynet.getenv("auth_token")

-- Start Croupier service
skynet.newservice("croupier_service")

-- Initialize Croupier client
local ok = skynet.call(croupier_service, "lua", "start", CROUPIER_SERVER, AUTH_TOKEN)
if not ok then
    skynet.error("Failed to start Croupier client")
    return
end

skynet.error("Croupier SDK initialized successfully")

-- Example: Register a virtual object
local function example_register_vo()
    local vo_id = "player:1001"
    local class_id = "player"
    local properties = {
        level = 50,
        vip_level = 3,
        coins = 10000
    }

    local ok, err = skynet.call(croupier_service, "lua", "register_vo", vo_id, class_id, properties)
    if ok then
        skynet.error(string.format("Registered VO: %s (class: %s)", vo_id, class_id))
    else
        skynet.error("Failed to register VO: " .. tostring(err))
    end
end

-- Example: Invoke a function
local function example_invoke()
    local vo_id = "player:1001"
    local function_name = "get_balance"
    local args = {currency = "coins"}

    local ok, result = skynet.call(croupier_service, "lua", "invoke", vo_id, function_name, args)
    if ok then
        skynet.error(string.format("Invoke result: %s", tostring(result)))
    else
        skynet.error("Failed to invoke: " .. tostring(result))
    end
end

-- Example: Batch invoke
local function example_batch_invoke()
    local requests = {
        {vo_id = "player:1001", function_name = "get_balance", args = {currency = "coins"}},
        {vo_id = "player:1001", function_name = "get_level", args = {}},
        {vo_id = "player:1001", function_name = "get_inventory", args = {}}
    }

    local ok, results = skynet.call(croupier_service, "lua", "batch_invoke", requests)
    if ok then
        skynet.error("Batch invoke completed")
        for i, result in ipairs(results) do
            skynet.error(string.format("  Result %d: %s", i, tostring(result)))
        end
    else
        skynet.error("Batch invoke failed: " .. tostring(results))
    end
end

-- Example: List virtual objects
local function example_list_vos()
    local ok, vos = skynet.call(croupier_service, "lua", "list_vos", "player")
    if ok then
        skynet.error("Found " .. #vos .. " player virtual objects")
        for i, vo_id in ipairs(vos) do
            skynet.error("  - " .. vo_id)
        end
    else
        skynet.error("Failed to list VOs: " .. tostring(vos))
    end
end

-- Run examples with delay
skynet.timeout(100, function()
    skynet.error("=== Running Croupier SDK Examples ===")
    example_register_vo()
end)

skynet.timeout(200, function()
    example_invoke()
end)

skynet.timeout(300, function()
    example_batch_invoke()
end)

skynet.timeout(400, function()
    example_list_vos()
end)

skynet.timeout(500, function()
    local status = skynet.call(croupier_service, "lua", "status")
    skynet.error(string.format("Service status: started=%s, requests=%d",
        tostring(status.started), status.request_count))
end)

-- Console command interface
skynet.dispatch("lua", function(session, address, cmd, ...)
    local f = {
        status = function()
            local status = skynet.call(croupier_service, "lua", "status")
            skynet.ret(skynet.pack(true, status))
        end,
        register = function(vo_id, class_id, ...)
            local properties = {...}
            local ok = skynet.call(croupier_service, "lua", "register_vo", vo_id, class_id, properties)
            skynet.ret(skynet.pack(ok))
        end,
        invoke = function(vo_id, func, ...)
            local args = {...}
            local ok, result = skynet.call(croupier_service, "lua", "invoke", vo_id, func, args)
            skynet.ret(skynet.pack(ok, result))
        end,
    }

    local func = f[cmd]
    if func then
        func(...)
    else
        skynet.ret(skynet.pack(false, "unknown command"))
    end
end)
