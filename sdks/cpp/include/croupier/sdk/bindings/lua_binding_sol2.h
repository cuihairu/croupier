// Copyright 2025 Croupier Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#ifndef CROUPIER_SDK_BINDINGS_LUA_BINDING_SOL2_H_
#define CROUPIER_SDK_BINDINGS_LUA_BINDING_SOL2_H_

// Forward declaration for lua_State (complete type defined in lua.h)
typedef struct lua_State lua_State;

#ifdef __cplusplus
extern "C" {
#endif

/**
 * @brief Lua binding entry point using sol2
 *
 * This function is called by Lua when require("croupier") is invoked.
 * It uses the sol2 library to bind CroupierClient and CroupierInvoker
 * classes to Lua in a type-safe manner.
 *
 * Usage in Lua:
 *   local croupier = require("croupier")
 *   local client = croupier.Client({server = "localhost:50051"})
 *
 * @param L Lua state pointer
 * @return Number of return values on Lua stack (always 1: the module table)
 */
int luaopen_croupier(lua_State* L);

#ifdef __cplusplus
}
#endif

#endif  // CROUPIER_SDK_BINDINGS_LUA_BINDING_SOL2_H_
