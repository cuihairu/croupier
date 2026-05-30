-- Croupier SDK Skynet Service
-- Copyright 2025 Croupier Authors
--
-- This is a Skynet service that wraps the Croupier SDK.
-- Usage:
--   1. Build the SDK with ENABLE_LUA_BINDING=ON
--   2. Copy libcroupier.so to your Skynet project's cservice directory
--   3. Start the service: skynet.call(croupier_service, "lua", "start", "localhost:50051")

local skynet = require "skynet"
require "skynet.manager"

local croupier = require "croupier"

-- Service state
local client = nil
local server_address = nil
local request_count = 0

-- ============================================================================
-- Service API
-- ============================================================================

local CMD = {}

--- Start the Croupier client
function CMD.start(address, auth_token)
    if client then
        skynet.error("[croupier] Client already started")
        return false, "already started"
    end

    server_address = address or "localhost:50051"
    client = croupier.Client.new(server_address)

    if auth_token then
        client:set_credentials(auth_token)
    end

    skynet.error("[croupier] Client started: " .. server_address)
    return true
end

--- Stop the Croupier client
function CMD.stop()
    if not client then
        return false, "not started"
    end

    client:close()
    client = nil
    server_address = nil
    request_count = 0

    skynet.error("[croupier] Client stopped")
    return true
end

--- Get service status
function CMD.status()
    return {
        started = client ~= nil,
        server_address = server_address,
        request_count = request_count
    }
end

--- Register a virtual object
function CMD.register_vo(vo_id, class_id, properties)
    if not client then
        return false, "client not started"
    end

    local ok, err = pcall(function()
        return client:register_virtual_object(vo_id, class_id, properties)
    end)

    if ok then
        request_count = request_count + 1
        return true
    else
        skynet.error("[croupier] Register VO failed: " .. tostring(err))
        return false, err
    end
end

--- Unregister a virtual object
function CMD.unregister_vo(vo_id)
    if not client then
        return false, "client not started"
    end

    local ok, err = pcall(function()
        return client:unregister_virtual_object(vo_id)
    end)

    if ok then
        request_count = request_count + 1
        return true
    else
        skynet.error("[croupier] Unregister VO failed: " .. tostring(err))
        return false, err
    end
end

--- Invoke a function on a virtual object
function CMD.invoke(vo_id, function_name, args)
    if not client then
        return false, "client not started"
    end

    local ok, result = pcall(function()
        return client:invoke_function(vo_id, function_name, args)
    end)

    if ok then
        request_count = request_count + 1
        return true, result
    else
        skynet.error("[croupier] Invoke failed: " .. tostring(result))
        return false, result
    end
end

--- Batch invoke multiple functions
function CMD.batch_invoke(requests)
    if not client then
        return false, "client not started"
    end

    -- Convert request array to proper format
    local formatted_requests = {}
    for i, req in ipairs(requests) do
        table.insert(formatted_requests, {
            vo_id = req.vo_id,
            function_name = req.function_name,
            args = croupier.json.encode(req.args or {})
        })
    end

    local json_requests = croupier.json.encode(formatted_requests)
    local ok, results = pcall(function()
        return client:batch_invoke(json_requests)
    end)

    if ok then
        request_count = request_count + #requests
        return true, results
    else
        skynet.error("[croupier] Batch invoke failed: " .. tostring(results))
        return false, results
    end
end

--- Get virtual object info
function CMD.get_vo(vo_id)
    if not client then
        return false, "client not started"
    end

    local ok, result = pcall(function()
        return client:get_virtual_object(vo_id)
    end)

    if ok then
        request_count = request_count + 1
        return true, result
    else
        skynet.error("[croupier] Get VO failed: " .. tostring(result))
        return false, result
    end
end

--- List virtual objects
function CMD.list_vos(class_id)
    if not client then
        return false, "client not started"
    end

    local ok, result = pcall(function()
        return client:list_virtual_objects(class_id)
    end)

    if ok then
        request_count = request_count + 1
        return true, result
    else
        skynet.error("[croupier] List VOs failed: " .. tostring(result))
        return false, result
    end
end

-- ============================================================================
-- Skynet Service Interface
-- ============================================================================

skynet.start(function()
    skynet.dispatch("lua", function(session, address, cmd, ...)
        local f = CMD[cmd]
        if f then
            local ok, ret1, ret2 = pcall(f, ...)
            if ok then
                if ret1 == false then
                    skynet.ret(skynet.pack(ret1, ret2))
                else
                    skynet.ret(skynet.pack(ret1))
                end
            else
                skynet.ret(skynet.pack(false, ret1))
            end
        else
            skynet.error("[croupier] Unknown command: " .. tostring(cmd))
            skynet.ret(skynet.pack(false, "unknown command"))
        end
    end)

    skynet.register(".croupier")
    skynet.error("[croupier] Service started")
end)
