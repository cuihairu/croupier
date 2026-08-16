-- Croupier SDK Skynet Service
-- Copyright 2025 Croupier Authors

local skynet = require "skynet"
require "skynet.manager"

local croupier = require "croupier"

local client = nil
local server_address = nil
local registered_functions = {}

local CMD = {}

function CMD.start(address, auth_token)
    if client then
        return false, "already started"
    end

    server_address = address or "127.0.0.1:19091"
    client = croupier.Client.new(server_address)

    if auth_token then
        client:set_credentials(auth_token)
    end

    skynet.error("[croupier] provider client created: " .. server_address)
    return true
end

function CMD.register_function(function_id, response_json)
    if not client then
        return false, "client not started"
    end
    if not function_id or function_id == "" then
        return false, "function_id is required"
    end

    local static_response = response_json or "{}"
    local ok, err = pcall(function()
        return client:register_function(function_id, function(_, _)
            return static_response
        end)
    end)

    if not ok then
        skynet.error("[croupier] register function failed: " .. tostring(err))
        return false, err
    end

    registered_functions[function_id] = true
    return true
end

function CMD.connect()
    if not client then
        return false, "client not started"
    end
    return client:connect()
end

function CMD.serve()
    if not client then
        return false, "client not started"
    end
    client:serve()
    return true
end

function CMD.stop()
    if not client then
        return false, "not started"
    end

    client:close()
    client = nil
    server_address = nil
    registered_functions = {}

    skynet.error("[croupier] provider client stopped")
    return true
end

function CMD.status()
    local count = 0
    for _ in pairs(registered_functions) do
        count = count + 1
    end

    return {
        started = client ~= nil,
        server_address = server_address,
        registered_function_count = count,
    }
end

skynet.start(function()
    skynet.dispatch("lua", function(_, _, cmd, ...)
        local f = CMD[cmd]
        if not f then
            skynet.error("[croupier] unknown command: " .. tostring(cmd))
            skynet.ret(skynet.pack(false, "unknown command"))
            return
        end

        local ok, ret1, ret2 = pcall(f, ...)
        if not ok then
            skynet.ret(skynet.pack(false, ret1))
            return
        end
        skynet.ret(skynet.pack(ret1, ret2))
    end)

    skynet.register(".croupier")
    skynet.error("[croupier] service started")
end)
