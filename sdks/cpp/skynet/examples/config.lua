-- Skynet Configuration for Croupier SDK Example
-- Copyright 2025 Croupier Authors

-- Thread settings
thread = 8
logger = nil
logpath = "."
harbor = 0
start = "main"  -- main control service

-- luaservice path: add croupier service directory
luaservice = root.."service/?.lua;"..root.."examples/?.lua;"..root.."test/?.lua"
lualoader = root.."lualoader.lua"
lua_path = root.."lualib/?.lua;"..root.."lualib/?/init.lua"
lua_cpath = root.."luaclib/?.so;"..root.."luaclib/?.dll;"..root.."../build/bin/?.so"

-- Snax service path (if needed)
snax = root.."examples/?.lua;"..root.."test/?.lua"

-- Bootstrap service
bootstrap = "snlua bootstrap"

-- C service path (include croupier C library)
cpath = root.."cservice/?.so;"..root.."../build/bin/?.so"
