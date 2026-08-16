-- Main service for Croupier SDK Skynet function provider example

local skynet = require "skynet"
local croupier_service = ".croupier"

local CROUPIER_AGENT = skynet.getenv("croupier_server") or "127.0.0.1:19091"
local AUTH_TOKEN = skynet.getenv("auth_token")

skynet.newservice("croupier_service")

local ok, err = skynet.call(croupier_service, "lua", "start", CROUPIER_AGENT, AUTH_TOKEN)
if not ok then
    skynet.error("Failed to start Croupier provider: " .. tostring(err))
    return
end

local function register(function_id, response_json)
    local registered, register_err = skynet.call(
        croupier_service,
        "lua",
        "register_function",
        function_id,
        response_json
    )
    if registered then
        skynet.error("Registered function: " .. function_id)
    else
        skynet.error("Failed to register function " .. function_id .. ": " .. tostring(register_err))
    end
end

skynet.timeout(100, function()
    skynet.error("=== Registering Croupier functions ===")
    register("player.profile.get", [[{"player_id":"player_1001","level":50,"vip_level":3}]])
    register("wallet.balance.get", [[{"player_id":"player_1001","currency":"coins","balance":10000}]])
    register("mail.send", [[{"mail_id":"mail_demo_001","status":"queued"}]])
end)

skynet.timeout(200, function()
    local connected, connect_err = skynet.call(croupier_service, "lua", "connect")
    if connected then
        skynet.error("Connected to Croupier Agent: " .. CROUPIER_AGENT)
    else
        skynet.error("Agent connection failed: " .. tostring(connect_err))
    end
end)

skynet.timeout(300, function()
    local status = skynet.call(croupier_service, "lua", "status")
    skynet.error(string.format(
        "Service status: started=%s, registered_functions=%d",
        tostring(status.started),
        status.registered_function_count
    ))
end)

skynet.dispatch("lua", function(_, _, cmd, ...)
    if cmd == "status" then
        local status = skynet.call(croupier_service, "lua", "status")
        skynet.ret(skynet.pack(true, status))
        return
    end

    if cmd == "register_function" then
        local registered, register_err = skynet.call(croupier_service, "lua", "register_function", ...)
        skynet.ret(skynet.pack(registered, register_err))
        return
    end

    skynet.ret(skynet.pack(false, "unknown command"))
end)
