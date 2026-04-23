-- Croupier SDK Skynet 简单示例
-- Copyright 2025 Croupier Authors
--
-- 这是最简单的示例，展示基本的用法
--
-- 运行方式:
--   ./skynet examples/simple_example.lua

local skynet = require "skynet"

-- 配置
local CROUPIER_SERVER = "localhost:50051"
local AUTH_TOKEN = nil  -- 如果需要认证，设置此值

print("========================================")
print("Croupier SDK Skynet 简单示例")
print("========================================")

-- 1. 启动 Croupier 服务
print("\n[1] 启动 Croupier 服务...")
local croupier_service = skynet.newservice("croupier_service")

local ok = skynet.call(croupier_service, "lua", "start", CROUPIER_SERVER, AUTH_TOKEN)
if not ok then
    print("✗ 启动失败！请检查：")
    print("  1. Croupier Server 是否运行在 " .. CROUPIER_SERVER)
    print("  2. libcroupier-sdk.so 是否在 cservice 目录")
    print("  3. Lua 模块是否正确安装")
    skynet.exit()
    return
end
print("✓ Croupier 服务已启动")

-- 2. 注册一个虚拟对象
print("\n[2] 注册虚拟对象...")
local vo_id = "demo_player_001"
local class_id = "player"
local properties = {
    name = "DemoPlayer",
    level = 10,
    coins = 1000,
    vip_level = 1
}

ok, err = skynet.call(croupier_service, "lua", "register_vo", vo_id, class_id, properties)
if ok then
    print("✓ 成功注册虚拟对象: " .. vo_id)
    print("  类型: " .. class_id)
    print("  属性: " .. properties.name .. " (Lv." .. properties.level .. ")")
else
    print("✗ 注册失败: " .. tostring(err))
end

-- 3. 调用函数
print("\n[3] 调用函数...")
local function_name = "get_balance"
local args = {currency = "coins"}

ok, result = skynet.call(croupier_service, "lua", "invoke", vo_id, function_name, args)
if ok then
    print("✓ 函数调用成功")
    print("  函数: " .. function_name)
    print("  结果: " .. tostring(result))
else
    print("✗ 函数调用失败: " .. tostring(result))
end

-- 4. 获取虚拟对象信息
print("\n[4] 获取虚拟对象信息...")
ok, vo = skynet.call(croupier_service, "lua", "get_vo", vo_id)
if ok then
    print("✓ 成功获取虚拟对象信息")
    print("  ID: " .. vo_id)
    -- 注意：返回的 vo 可能包含其他信息
else
    print("✗ 获取失败: " .. tostring(vo))
end

-- 5. 查看服务状态
print("\n[5] 服务状态...")
local status = skynet.call(croupier_service, "lua", "status")
print("✓ 服务状态:")
print("  已启动: " .. tostring(status.started))
print("  服务器: " .. (status.server_address or "unknown"))
print("  请求次数: " .. tostring(status.request_count))

-- 6. 清理
print("\n[6] 清理资源...")
ok = skynet.call(croupier_service, "lua", "stop")
if ok then
    print("✓ 服务已停止")
else
    print("✗ 停止服务失败")
end

print("\n========================================")
print("示例运行完成！")
print("========================================")

-- 等待一段时间后退出
skynet.timeout(100, function()
    print("\n即将退出...")
    skynet.exit()
end)
