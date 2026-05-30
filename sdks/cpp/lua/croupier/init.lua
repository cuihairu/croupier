-- Croupier SDK Lua Module
-- Copyright 2025 Croupier Authors
--
-- This module provides a Lua interface to the Croupier SDK.
-- It loads the C binding and provides a clean Lua API.

local M = {}

-- Version information
M.VERSION = "0.1.0"
M.SDK_VERSION = CROUPIER_SDK_VERSION or "0.1.0"

-- Load C binding
local ok, binding = pcall(require, "croupier.core")
if not ok then
    -- Try loading the compiled module directly
    ok, binding = pcall(require, "croupier_sdk")
    if not ok then
        error("Failed to load Croupier C binding: " .. (binding or "unknown error"))
    end
end

-- Export Client class
M.Client = binding.Client

-- Export version
M._VERSION = M.VERSION

-- Convenience function for creating a new client
function M.new(server_address)
    return M.Client.new(server_address)
end

-- Helper for JSON operations (using cjson if available)
local json_ok, json = pcall(require, "cjson")
if json_ok then
    M.json = json
else
    -- Simple fallback JSON encoder/decoder
    M.json = {
        encode = function(t)
            -- Basic JSON encoding for simple tables
            -- For production, use lua-cjson or similar
            local function serialize(val)
                local type_val = type(val)
                if type_val == "string" then
                    return '"' .. val .. '"'
                elseif type_val == "number" then
                    return tostring(val)
                elseif type_val == "boolean" then
                    return val and "true" or "false"
                elseif type_val == "table" then
                    local is_array = true
                    local max_index = 0
                    for k, v in pairs(val) do
                        if type(k) ~= "number" or k <= 0 or math.floor(k) ~= k then
                            is_array = false
                            break
                        end
                        max_index = math.max(max_index, k)
                    end

                    if is_array and max_index > 0 then
                        local parts = {}
                        for i = 1, max_index do
                            parts[i] = serialize(val[i])
                        end
                        return "[" .. table.concat(parts, ",") .. "]"
                    else
                        local parts = {}
                        for k, v in pairs(val) do
                            table.insert(parts, '"' .. k .. '":' .. serialize(v))
                        end
                        return "{" .. table.concat(parts, ",") .. "}"
                    end
                elseif type_val == "nil" then
                    return "null"
                else
                    return "null"
                end
            end
            return serialize(t)
        end,
        decode = function(str)
            -- Basic JSON parsing - for production use lua-cjson
            -- This is a minimal implementation
            return load("return " .. str)()
        end
    }
end

-- Helper for creating virtual object registration data
function M.create_vo_data(vo_id, class_id, properties)
    properties = properties or {}
    return {
        vo_id = vo_id,
        class_id = class_id,
        properties = properties
    }
end

-- Helper for creating function invocation request
function M.create_invoke_request(vo_id, function_name, args)
    args = args or {}
    return {
        vo_id = vo_id,
        function_name = function_name,
        args = args
    }
end

return M
