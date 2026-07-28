-- Standalone Lua Example for Croupier SDK
--
-- Usage:
--   lua -e "package.cpath = './build/bin/?.so;' .. package.cpath" lua/examples/standalone_example.lua

local croupier = require "croupier"

print("=== Croupier SDK Lua Function Example ===")
print("SDK Version: " .. croupier.SDK_VERSION)

local client = croupier.Client.new("127.0.0.1:19090")
print("Client created: " .. tostring(client))

local ok = client:register_function("lua.demo.echo", function(_, payload)
    return payload
end)

if ok then
    print("Registered function: lua.demo.echo")
else
    print("Failed to register function: lua.demo.echo")
end

local connected = client:connect()
if connected then
    print("Connected to Croupier Agent")
else
    print("Agent connection failed; this is expected when no local Agent is running")
end

client:close()
print("Client closed")

local invoker = croupier.Invoker.new("127.0.0.1:19090")
print("Invoker created: " .. tostring(invoker))
invoker:close()

print("=== Example Complete ===")
