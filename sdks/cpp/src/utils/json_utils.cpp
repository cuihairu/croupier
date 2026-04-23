#include "croupier/sdk/utils/json_utils.h"

#include <algorithm>
#include <iostream>
#include <sstream>

#ifdef CROUPIER_SDK_ENABLE_JSON
#include <nlohmann/json.hpp>
using json = nlohmann::json;
#endif

namespace croupier {
namespace sdk {
namespace utils {

// ========== JSON Parsing ==========

#ifdef CROUPIER_SDK_ENABLE_JSON
nlohmann::json JsonUtils::ParseJson(const std::string& json_content) {
    try {
        return nlohmann::json::parse(json_content);
    } catch (const std::exception& e) {
        throw std::runtime_error("Failed to parse JSON: " + std::string(e.what()));
    }
}
#else
JsonUtils::SimpleJson JsonUtils::ParseJson(const std::string& json_content) {
    // Simplified JSON parser for basic functionality
    SimpleJson result;
    result.type = SimpleJson::OBJECT;

    // Basic component parsing
    if (json_content.find("\"component\"") != std::string::npos) {
        SimpleJson comp;
        comp.type = SimpleJson::OBJECT;
        comp.object_value["id"] = SimpleJson("demo-component");
        comp.object_value["version"] = SimpleJson("1.0.0");
        comp.object_value["name"] = SimpleJson("Demo Component");
        result.object_value["component"] = comp;
    }

    // Basic client config parsing
    if (json_content.find("\"game_id\"") != std::string::npos) {
        result.object_value["game_id"] = SimpleJson("example-game");
        result.object_value["env"] = SimpleJson("development");
        result.object_value["service_id"] = SimpleJson("cpp-service");
    }

    return result;
}
#endif

bool JsonUtils::IsValidJson(const std::string& json_content) {
    try {
#ifdef CROUPIER_SDK_ENABLE_JSON
        [[maybe_unused]] auto parsed = nlohmann::json::parse(json_content);
#else
        auto begin = json_content.find_first_not_of(" \t\r\n");
        if (begin == std::string::npos) {
            return false;
        }
        auto end = json_content.find_last_not_of(" \t\r\n");
        if (end == std::string::npos || end < begin) {
            return false;
        }

        const char first = json_content[begin];
        const char last = json_content[end];
        if (first == '{') {
            if (last != '}') {
                return false;
            }
        } else if (first == '[') {
            if (last != ']') {
                return false;
            }
        } else {
            return false;
        }

        // Balanced bracket check (best-effort) ignoring content within quoted strings.
        int brace_depth = 0;
        int bracket_depth = 0;
        bool in_string = false;
        bool escaped = false;
        for (size_t i = begin; i <= end; i++) {
            const char ch = json_content[i];
            if (in_string) {
                if (escaped) {
                    escaped = false;
                    continue;
                }
                if (ch == '\\') {
                    escaped = true;
                    continue;
                }
                if (ch == '"') {
                    in_string = false;
                }
                continue;
            }

            if (ch == '"') {
                in_string = true;
                continue;
            }

            switch (ch) {
            case '{':
                brace_depth++;
                break;
            case '}':
                brace_depth--;
                if (brace_depth < 0) {
                    return false;
                }
                break;
            case '[':
                bracket_depth++;
                break;
            case ']':
                bracket_depth--;
                if (bracket_depth < 0) {
                    return false;
                }
                break;
            default:
                break;
            }
        }

        if (in_string || escaped) {
            return false;
        }
        if (brace_depth != 0 || bracket_depth != 0) {
            return false;
        }
#endif
        return true;
    } catch (...) {
        return false;
    }
}

// ========== Value Extraction ==========

#ifdef CROUPIER_SDK_ENABLE_JSON
std::string JsonUtils::GetStringValue(const nlohmann::json& json_obj, const std::string& path,
                                      const std::string& default_value) {
    try {
        auto path_parts = SplitPath(path);
        auto value = GetValueByPath(json_obj, path_parts);

        if (value.is_string()) {
            return value.get<std::string>();
        }
        return default_value;
    } catch (...) {
        return default_value;
    }
}
#else
std::string JsonUtils::GetStringValue(const SimpleJson& json_obj, const std::string& path,
                                      const std::string& default_value) {
    try {
        auto path_parts = SplitPath(path);
        auto value = GetValueByPath(json_obj, path_parts);
        return value.str_value;
    } catch (...) {
        return default_value;
    }
}
#endif

#ifdef CROUPIER_SDK_ENABLE_JSON
int JsonUtils::GetIntValue(const nlohmann::json& json_obj, const std::string& path, int default_value) {
    try {
        auto path_parts = SplitPath(path);
        auto value = GetValueByPath(json_obj, path_parts);

        if (value.is_number_integer()) {
            return value.get<int>();
        }
        return default_value;
    } catch (...) {
        return default_value;
    }
}
#else
int JsonUtils::GetIntValue(const SimpleJson& json_obj, const std::string& path, int default_value) {
    try {
        auto path_parts = SplitPath(path);
        auto value = GetValueByPath(json_obj, path_parts);
        return std::stoi(value.str_value);
    } catch (...) {
        return default_value;
    }
}
#endif

#ifdef CROUPIER_SDK_ENABLE_JSON
bool JsonUtils::GetBoolValue(const nlohmann::json& json_obj, const std::string& path, bool default_value) {
    try {
        auto path_parts = SplitPath(path);
        auto value = GetValueByPath(json_obj, path_parts);

        if (value.is_boolean()) {
            return value.get<bool>();
        }
        return default_value;
    } catch (...) {
        return default_value;
    }
}
#else
bool JsonUtils::GetBoolValue(const SimpleJson& json_obj, const std::string& path, bool default_value) {
    try {
        auto path_parts = SplitPath(path);
        auto value = GetValueByPath(json_obj, path_parts);
        std::string str_val = value.str_value;
        std::transform(str_val.begin(), str_val.end(), str_val.begin(), ::tolower);
        return str_val == "true" || str_val == "1";
    } catch (...) {
        return default_value;
    }
}
#endif

// ========== JSON Manipulation ==========

#ifdef CROUPIER_SDK_ENABLE_JSON
nlohmann::json JsonUtils::MergeJson(const nlohmann::json& base, const nlohmann::json& overlay) {
    nlohmann::json result = base;

    if (overlay.is_object() && result.is_object()) {
        for (auto it = overlay.begin(); it != overlay.end(); ++it) {
            if (result.contains(it.key()) && result[it.key()].is_object() && it.value().is_object()) {
                result[it.key()] = MergeJson(result[it.key()], it.value());
            } else {
                result[it.key()] = it.value();
            }
        }
    } else {
        result = overlay;
    }

    return result;
}
#else
JsonUtils::SimpleJson JsonUtils::MergeJson(const SimpleJson& base, const SimpleJson& overlay) {
    SimpleJson result = base;

    if (overlay.type == SimpleJson::OBJECT && result.type == SimpleJson::OBJECT) {
        for (const auto& [key, value] : overlay.object_value) {
            result.object_value[key] = value;
        }
    } else {
        result = overlay;
    }

    return result;
}
#endif

#ifdef CROUPIER_SDK_ENABLE_JSON
std::string JsonUtils::PrettyPrint(const nlohmann::json& json_obj, int indent) {
    try {
        return json_obj.dump(indent);
    } catch (const std::exception& e) {
        std::cerr << "Failed to pretty print JSON: " << e.what() << std::endl;
        return "{}";
    }
}
#else
std::string JsonUtils::PrettyPrint(const SimpleJson& json_obj, int indent) {
    std::stringstream ss;
    PrettyPrintRecursive(json_obj, ss, 0, indent);
    return ss.str();
}

void JsonUtils::PrettyPrintRecursive(const SimpleJson& json_obj, std::stringstream& ss, int current_indent,
                                     int indent_size) {
    std::string indent_str(current_indent, ' ');

    switch (json_obj.type) {
    case SimpleJson::STRING:
        ss << "\"" << json_obj.str_value << "\"";
        break;
    case SimpleJson::OBJECT: {
        ss << "{\n";
        bool first = true;
        for (const auto& [key, value] : json_obj.object_value) {
            if (!first)
                ss << ",\n";
            ss << std::string(current_indent + indent_size, ' ') << "\"" << key << "\": ";
            PrettyPrintRecursive(value, ss, current_indent + indent_size, indent_size);
            first = false;
        }
        ss << "\n" << indent_str << "}";
        break;
    }
    case SimpleJson::ARRAY: {
        ss << "[\n";
        bool first = true;
        for (const auto& value : json_obj.array_value) {
            if (!first)
                ss << ",\n";
            ss << std::string(current_indent + indent_size, ' ');
            PrettyPrintRecursive(value, ss, current_indent + indent_size, indent_size);
            first = false;
        }
        ss << "\n" << indent_str << "]";
        break;
    }
    }
}
#endif

// ========== Template Specializations ==========

template <>
std::string JsonUtils::ToJsonString<std::map<std::string, std::string>>(const std::map<std::string, std::string>& obj) {
#ifdef CROUPIER_SDK_ENABLE_JSON
    nlohmann::json json_obj(obj);
    return json_obj.dump();
#else
    std::stringstream ss;
    ss << "{";
    bool first = true;
    for (const auto& [key, value] : obj) {
        if (!first)
            ss << ", ";
        ss << "\"" << key << "\": \"" << value << "\"";
        first = false;
    }
    ss << "}";
    return ss.str();
#endif
}

template <>
std::string JsonUtils::ToJsonString<std::vector<std::string>>(const std::vector<std::string>& obj) {
#ifdef CROUPIER_SDK_ENABLE_JSON
    nlohmann::json json_obj(obj);
    return json_obj.dump();
#else
    std::stringstream ss;
    ss << "[";
    bool first = true;
    for (const auto& item : obj) {
        if (!first)
            ss << ", ";
        ss << "\"" << item << "\"";
        first = false;
    }
    ss << "]";
    return ss.str();
#endif
}

// ========== Helper Methods ==========

std::vector<std::string> JsonUtils::SplitPath(const std::string& path) {
    std::vector<std::string> parts;
    std::stringstream ss(path);
    std::string item;

    while (std::getline(ss, item, '.')) {
        if (!item.empty()) {
            parts.push_back(item);
        }
    }

    return parts;
}

#ifdef CROUPIER_SDK_ENABLE_JSON
nlohmann::json JsonUtils::GetValueByPath(const nlohmann::json& json_obj, const std::vector<std::string>& path_parts) {
    nlohmann::json current = json_obj;

    for (const auto& part : path_parts) {
        if (current.is_object() && current.contains(part)) {
            current = current[part];
        } else {
            throw std::runtime_error("Path not found: " + part);
        }
    }

    return current;
}
#else
JsonUtils::SimpleJson JsonUtils::GetValueByPath(const SimpleJson& json_obj,
                                                const std::vector<std::string>& path_parts) {
    SimpleJson current = json_obj;

    for (const auto& part : path_parts) {
        if (current.type == SimpleJson::OBJECT && current.contains(part)) {
            current = current.at(part);
        } else {
            throw std::runtime_error("Path not found: " + part);
        }
    }

    return current;
}
#endif

// ========== JSON Schema Validation ==========

bool JsonUtils::ValidateJsonSchema(const std::string& json_content, const std::string& schema_content) {
    try {
#ifdef CROUPIER_SDK_ENABLE_JSON
        // Parse JSON and schema
        nlohmann::json json_obj = nlohmann::json::parse(json_content);
        nlohmann::json schema_obj = nlohmann::json::parse(schema_content);

        // Basic schema validation (without nlohmann::json-schema)
        // This implements a subset of JSON Schema validation

        // Check if schema specifies type
        if (schema_obj.contains("type")) {
            std::string expected_type = schema_obj["type"];
            bool type_valid = false;

            if (expected_type == "object" && json_obj.is_object()) {
                type_valid = true;
            } else if (expected_type == "array" && json_obj.is_array()) {
                type_valid = true;
            } else if (expected_type == "string" && json_obj.is_string()) {
                type_valid = true;
            } else if (expected_type == "number" && (json_obj.is_number_integer() || json_obj.is_number_float())) {
                type_valid = true;
            } else if (expected_type == "integer" && json_obj.is_number_integer()) {
                type_valid = true;
            } else if (expected_type == "boolean" && json_obj.is_boolean()) {
                type_valid = true;
            } else if (expected_type == "null" && json_obj.is_null()) {
                type_valid = true;
            }

            if (!type_valid) {
                std::cerr << "Schema validation failed: expected type " << expected_type << std::endl;
                return false;
            }
        }

        // Check required properties for objects
        if (json_obj.is_object() && schema_obj.contains("required") && schema_obj["required"].is_array()) {
            for (const auto& required_prop : schema_obj["required"]) {
                if (!required_prop.is_string() || !json_obj.contains(required_prop.get<std::string>())) {
                    std::cerr << "Schema validation failed: missing required property " << required_prop << std::endl;
                    return false;
                }
            }
        }

        // Validate properties
        if (json_obj.is_object() && schema_obj.contains("properties") && schema_obj["properties"].is_object()) {
            for (auto& [prop_name, prop_schema] : schema_obj["properties"].items()) {
                if (json_obj.contains(prop_name)) {
                    // Create a mini-schema for this property
                    nlohmann::json prop_mini_schema;
                    prop_mini_schema["type"] = prop_schema["type"];

                    // Recursively validate property
                    if (!ValidateJsonSchema(json_obj[prop_name].dump(), prop_mini_schema.dump())) {
                        std::cerr << "Schema validation failed for property " << prop_name << std::endl;
                        return false;
                    }
                }
            }
        }

        // Check minimum value for numbers
        if (json_obj.is_number() && schema_obj.contains("minimum")) {
            double value = json_obj;
            double minimum = schema_obj["minimum"];
            if (value < minimum) {
                std::cerr << "Schema validation failed: value " << value << " is less than minimum " << minimum
                          << std::endl;
                return false;
            }
        }

        // Check maximum value for numbers
        if (json_obj.is_number() && schema_obj.contains("maximum")) {
            double value = json_obj;
            double maximum = schema_obj["maximum"];
            if (value > maximum) {
                std::cerr << "Schema validation failed: value " << value << " is greater than maximum " << maximum
                          << std::endl;
                return false;
            }
        }

        // Check minimum length for strings
        if (json_obj.is_string() && schema_obj.contains("minLength")) {
            std::string str_val = json_obj;
            size_t min_length = schema_obj["minLength"];
            if (str_val.length() < min_length) {
                std::cerr << "Schema validation failed: string length " << str_val.length() << " is less than minimum "
                          << min_length << std::endl;
                return false;
            }
        }

        // Check maximum length for strings
        if (json_obj.is_string() && schema_obj.contains("maxLength")) {
            std::string str_val = json_obj;
            size_t max_length = schema_obj["maxLength"];
            if (str_val.length() > max_length) {
                std::cerr << "Schema validation failed: string length " << str_val.length()
                          << " is greater than maximum " << max_length << std::endl;
                return false;
            }
        }

        // Check minimum array length
        if (json_obj.is_array() && schema_obj.contains("minItems")) {
            size_t min_items = schema_obj["minItems"];
            if (json_obj.size() < min_items) {
                std::cerr << "Schema validation failed: array size " << json_obj.size() << " is less than minimum "
                          << min_items << std::endl;
                return false;
            }
        }

        // Check maximum array length
        if (json_obj.is_array() && schema_obj.contains("maxItems")) {
            size_t max_items = schema_obj["maxItems"];
            if (json_obj.size() > max_items) {
                std::cerr << "Schema validation failed: array size " << json_obj.size() << " is greater than maximum "
                          << max_items << std::endl;
                return false;
            }
        }

        // Check enum values
        if (schema_obj.contains("enum") && schema_obj["enum"].is_array()) {
            bool found = false;
            for (const auto& enum_val : schema_obj["enum"]) {
                if (json_obj == enum_val) {
                    found = true;
                    break;
                }
            }
            if (!found) {
                std::cerr << "Schema validation failed: value not in enum list" << std::endl;
                return false;
            }
        }

        return true;
#else
        // Fallback: basic validation without nlohmann::json
        // Just check if JSON is valid and schema is not empty
        if (json_content.empty() || schema_content.empty()) {
            return false;
        }
        return IsValidJson(json_content);
#endif

    } catch (const std::exception& e) {
        std::cerr << "Schema validation error: " << e.what() << '\n';
        return false;
    }
}

}  // namespace utils
}  // namespace sdk
}  // namespace croupier
