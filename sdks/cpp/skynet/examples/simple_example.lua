-- Croupier SDK Skynet simple provider example

local skynet = require "skynet"

local CROUPIER_AGENT = skynet.getenv("croupier_server") or "127.0.0.1:19091"
local AUTH_TOKEN = skynet.getenv("auth_token")

print("========================================")
print("Croupier SDK Skynet 函数注册示例")
print("========================================")

print("\n[1] 启动 Croupier Provider 服务...")
local croupier_service = skynet.newservice("croupier_service")

local ok, err = skynet.call(croupier_service, "lua", "start", CROUPIER_AGENT, AUTH_TOKEN)
if not ok then
    print("启动失败: " .. tostring(err))
    skynet.exit()
    return
end

print("\n[2] 注册函数...")
ok, err = skynet.call(
    croupier_service,
    "lua",
    "register_function",
    "player.profile.get",
    [[{"player_id":"demo_player_001","name":"DemoPlayer","level":10}]]
)
if not ok then
    print("注册失败: " .. tostring(err))
    skynet.exit()
    return
end
print("函数已注册: player.profile.get")

print("\n[3] 连接 Agent...")
ok, err = skynet.call(croupier_service, "lua", "connect")
if ok then
    print("已连接 Agent: " .. CROUPIER_AGENT)
else
    print("连接失败，这在本地未启动 Agent 时是正常的: " .. tostring(err))
end

print("\n[4] 服务状态...")
local status = skynet.call(croupier_service, "lua", "status")
print("已启动: " .. tostring(status.started))
print("注册函数数: " .. tostring(status.registered_function_count))

print("\n[5] 清理资源...")
skynet.call(croupier_service, "lua", "stop")
print("示例运行完成")

skynet.timeout(100, function()
    skynet.exit()
end)
