-- Standalone Lua Example for Croupier SDK
-- Copyright 2025 Croupier Authors
--
-- Usage:
--   lua -e "package.cpath = './build/bin/?.so;' .. package.cpath" examples/standalone_example.lua

local croupier = require "croupier"

print("=== Croupier SDK Standalone Example ===")
print("SDK Version: " .. croupier.SDK_VERSION)

-- Create client
local client = croupier.Client.new("localhost:50051")
print("Client created: " .. tostring(client))

-- Register a virtual object
print("\n--- Register Virtual Object ---")
local vo_id = "lua:test_vo_001"
local class_id = "test_class"
local properties = {
    name = "Test Object",
    value = 42,
    active = true
}

local ok = client:register_virtual_object(vo_id, class_id, properties)
if ok then
    print("Registered: " .. vo_id)
else
    print("Failed to register: " .. vo_id)
end

-- Get virtual object
print("\n--- Get Virtual Object ---")
local vo_data = client:get_virtual_object(vo_id)
print("VO Data: " .. tostring(vo_data))

-- Invoke function
print("\n--- Invoke Function ---")
local result = client:invoke_function(vo_id, "get_value", {})
print("Invoke result: " .. tostring(result))

-- Batch invoke
print("\n--- Batch Invoke ---")
local json = croupier.json.encode({
    {vo_id = vo_id, function_name = "get_value", args = {}},
    {vo_id = vo_id, function_name = "get_status", args = {}}
})
local batch_results = client:batch_invoke(json)
for i, r in ipairs(batch_results) do
    print("Batch result " .. i .. ": " .. tostring(r))
end

-- List virtual objects
print("\n--- List Virtual Objects ---")
local vos = client:list_virtual_objects()
if vos then
    print("Found " .. #vos .. " VOs")
    for i, id in ipairs(vos) do
        print("  " .. i .. ": " .. id)
    end
end

-- Unregister
print("\n--- Unregister Virtual Object ---")
ok = client:unregister_virtual_object(vo_id)
if ok then
    print("Unregistered: " .. vo_id)
end

-- Close client
print("\n--- Close Client ---")
client:close()
print("Client closed")

print("\n=== Example Complete ===")
