// Croupier SDK field hints helper (x-ui-* presentation contract).
//
// 契约：docs/architecture/presentation-hints.md
// 用法：
//   FunctionDescriptor desc = ...;
//   croupier::sdk::SetFieldWidget(desc, "id", "Select");
//   croupier::sdk::SetFieldHint(desc, "id", "x-options-source", {{"functionId","player.list"}});
//
// 依赖 nlohmann/json（vcpkg PUBLIC 依赖，SDK 使用方已可用）。

#pragma once

#include "croupier/sdk/croupier_client.h"

#include <nlohmann/json.hpp>

#include <cctype>
#include <string>

namespace croupier::sdk {

// 向 input_schema 的 properties[field] 合并单个 x-* hint。
// schema 为空时创建 object 骨架；重复设置覆盖；
// hint 非法（非 x-/x_ 前缀）或 field 为空返回 false 且不修改 descriptor。
inline bool SetFieldHint(FunctionDescriptor& descriptor, const std::string& field,
                         const std::string& hint, const nlohmann::json& value) {
    if (field.empty() || hint.size() < 3) {
        return false;
    }
    const char c0 = static_cast<char>(std::tolower(static_cast<unsigned char>(hint[0])));
    if (c0 != 'x' || (hint[1] != '-' && hint[1] != '_')) {
        return false;
    }
    const std::string normalized = "x-" + hint.substr(2);
    if (normalized.size() < 3) {
        return false;
    }

    nlohmann::json schema = nlohmann::json::object();
    if (!descriptor.input_schema.empty()) {
        try {
            schema = nlohmann::json::parse(descriptor.input_schema);
        } catch (...) {
            return false;
        }
        if (!schema.is_object()) {
            return false;
        }
    }
    if (!schema.contains("type")) {
        schema["type"] = "object";
    }
    if (!schema.contains("properties") || !schema["properties"].is_object()) {
        schema["properties"] = nlohmann::json::object();
    }
    nlohmann::json& properties = schema["properties"];
    if (!properties.contains(field) || !properties[field].is_object()) {
        properties[field] = nlohmann::json::object();
    }
    properties[field][normalized] = value;
    descriptor.input_schema = schema.dump();
    return true;
}

// 等价于 SetFieldHint(descriptor, field, "x-widget", widget)。
inline bool SetFieldWidget(FunctionDescriptor& descriptor, const std::string& field,
                           const std::string& widget) {
    if (widget.empty()) {
        return false;
    }
    return SetFieldHint(descriptor, field, "x-widget", nlohmann::json(widget));
}

}  // namespace croupier::sdk
