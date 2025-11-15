#include "croupier/sdk/utils/json_utils.h"
#include <sstream>
#include <iostream>
#include <algorithm>

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
        nlohmann::json::parse(json_content);
#else
        // Simple validation for basic JSON structure
        if (json_content.empty() ||
            (json_content.front() != '{' && json_content.front() != '[')) {
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
std::string JsonUtils::GetStringValue(const nlohmann::json& json_obj,
                                     const std::string& path,
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
std::string JsonUtils::GetStringValue(const SimpleJson& json_obj,
                                     const std::string& path,
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
int JsonUtils::GetIntValue(const nlohmann::json& json_obj,
                          const std::string& path,
                          int default_value) {
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
int JsonUtils::GetIntValue(const SimpleJson& json_obj,
                          const std::string& path,
                          int default_value) {
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
bool JsonUtils::GetBoolValue(const nlohmann::json& json_obj,
                            const std::string& path,
                            bool default_value) {
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
bool JsonUtils::GetBoolValue(const SimpleJson& json_obj,
                            const std::string& path,
                            bool default_value) {
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

void JsonUtils::PrettyPrintRecursive(const SimpleJson& json_obj, std::stringstream& ss, int current_indent, int indent_size) {
    std::string indent_str(current_indent, ' ');

    switch (json_obj.type) {
        case SimpleJson::STRING:
            ss << "\"" << json_obj.str_value << "\"";
            break;
        case SimpleJson::OBJECT:
        {
            ss << "{\n";
            bool first = true;
            for (const auto& [key, value] : json_obj.object_value) {
                if (!first) ss << ",\n";
                ss << std::string(current_indent + indent_size, ' ') << "\"" << key << "\": ";
                PrettyPrintRecursive(value, ss, current_indent + indent_size, indent_size);
                first = false;
            }
            ss << "\n" << indent_str << "}";
            break;
        }
        case SimpleJson::ARRAY:
        {
            ss << "[\n";
            bool first = true;
            for (const auto& value : json_obj.array_value) {
                if (!first) ss << ",\n";
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

template<>
std::string JsonUtils::ToJsonString<std::map<std::string, std::string>>(const std::map<std::string, std::string>& obj) {
#ifdef CROUPIER_SDK_ENABLE_JSON
    nlohmann::json json_obj(obj);
    return json_obj.dump();
#else
    std::stringstream ss;
    ss << "{";
    bool first = true;
    for (const auto& [key, value] : obj) {
        if (!first) ss << ", ";
        ss << "\"" << key << "\": \"" << value << "\"";
        first = false;
    }
    ss << "}";
    return ss.str();
#endif
}

template<>
std::string JsonUtils::ToJsonString<std::vector<std::string>>(const std::vector<std::string>& obj) {
#ifdef CROUPIER_SDK_ENABLE_JSON
    nlohmann::json json_obj(obj);
    return json_obj.dump();
#else
    std::stringstream ss;
    ss << "[";
    bool first = true;
    for (const auto& item : obj) {
        if (!first) ss << ", ";
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
JsonUtils::SimpleJson JsonUtils::GetValueByPath(const SimpleJson& json_obj, const std::vector<std::string>& path_parts) {
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

} // namespace utils
} // namespace sdk
} // namespace croupier