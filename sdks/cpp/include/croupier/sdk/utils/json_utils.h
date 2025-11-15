#pragma once

#include <string>
#include <map>
#include <vector>
#include <stdexcept>

#ifdef CROUPIER_SDK_ENABLE_JSON
#include <nlohmann/json.hpp>
#endif

namespace croupier {
namespace sdk {
namespace utils {

/**
 * @brief JSON processing utility class
 *
 * Provides unified JSON parsing, validation, and manipulation functions
 */
class JsonUtils {
public:
    /**
     * @brief Parse JSON string to internal representation
     *
     * @param json_content JSON string content
     * @return parsed JSON object (nlohmann::json or fallback type)
     * @throws std::runtime_error if parsing fails
     */
#ifdef CROUPIER_SDK_ENABLE_JSON
    static nlohmann::json ParseJson(const std::string& json_content);
#else
    struct SimpleJson {
        std::string str_value;
        std::map<std::string, SimpleJson> object_value;
        std::vector<SimpleJson> array_value;
        enum Type { STRING, OBJECT, ARRAY } type;

        SimpleJson() : type(STRING) {}
        SimpleJson(const std::string& s) : str_value(s), type(STRING) {}

        bool contains(const std::string& key) const {
            return type == OBJECT && object_value.find(key) != object_value.end();
        }

        const SimpleJson& at(const std::string& key) const {
            if (type != OBJECT) throw std::runtime_error("Not an object");
            auto it = object_value.find(key);
            if (it == object_value.end()) throw std::runtime_error("Key not found: " + key);
            return it->second;
        }

        std::string value(const std::string& key, const std::string& default_val) const {
            if (contains(key)) {
                return at(key).str_value;
            }
            return default_val;
        }
    };

    static SimpleJson ParseJson(const std::string& json_content);
#endif

    /**
     * @brief Validate JSON string format
     *
     * @param json_content JSON string to validate
     * @return true if valid JSON format
     */
    static bool IsValidJson(const std::string& json_content);

    /**
     * @brief Convert object to JSON string
     *
     * @param obj Object to convert (template for various types)
     * @return JSON string representation
     */
    template<typename T>
    static std::string ToJsonString(const T& obj);

    /**
     * @brief Extract string value from JSON path
     *
     * @param json_obj JSON object
     * @param path Dot-separated path (e.g., "config.database.url")
     * @param default_value Default value if path doesn't exist
     * @return String value
     */
#ifdef CROUPIER_SDK_ENABLE_JSON
    static std::string GetStringValue(const nlohmann::json& json_obj,
                                    const std::string& path,
                                    const std::string& default_value = "");
#else
    static std::string GetStringValue(const SimpleJson& json_obj,
                                    const std::string& path,
                                    const std::string& default_value = "");
#endif

    /**
     * @brief Extract integer value from JSON path
     *
     * @param json_obj JSON object
     * @param path Dot-separated path
     * @param default_value Default value if path doesn't exist
     * @return Integer value
     */
#ifdef CROUPIER_SDK_ENABLE_JSON
    static int GetIntValue(const nlohmann::json& json_obj,
                          const std::string& path,
                          int default_value = 0);
#else
    static int GetIntValue(const SimpleJson& json_obj,
                          const std::string& path,
                          int default_value = 0);
#endif

    /**
     * @brief Extract boolean value from JSON path
     *
     * @param json_obj JSON object
     * @param path Dot-separated path
     * @param default_value Default value if path doesn't exist
     * @return Boolean value
     */
#ifdef CROUPIER_SDK_ENABLE_JSON
    static bool GetBoolValue(const nlohmann::json& json_obj,
                            const std::string& path,
                            bool default_value = false);
#else
    static bool GetBoolValue(const SimpleJson& json_obj,
                            const std::string& path,
                            bool default_value = false);
#endif

    /**
     * @brief Merge two JSON objects
     *
     * @param base Base JSON object
     * @param overlay Overlay JSON object (values override base)
     * @return Merged JSON object
     */
#ifdef CROUPIER_SDK_ENABLE_JSON
    static nlohmann::json MergeJson(const nlohmann::json& base, const nlohmann::json& overlay);
#else
    static SimpleJson MergeJson(const SimpleJson& base, const SimpleJson& overlay);
#endif

    /**
     * @brief Pretty print JSON with indentation
     *
     * @param json_obj JSON object to format
     * @param indent Indentation spaces (default: 2)
     * @return Formatted JSON string
     */
#ifdef CROUPIER_SDK_ENABLE_JSON
    static std::string PrettyPrint(const nlohmann::json& json_obj, int indent = 2);
#else
    static std::string PrettyPrint(const SimpleJson& json_obj, int indent = 2);
#endif

private:
    // Helper methods for path parsing
    static std::vector<std::string> SplitPath(const std::string& path);

#ifdef CROUPIER_SDK_ENABLE_JSON
    static nlohmann::json GetValueByPath(const nlohmann::json& json_obj, const std::vector<std::string>& path_parts);
#else
    static SimpleJson GetValueByPath(const SimpleJson& json_obj, const std::vector<std::string>& path_parts);
    static void PrettyPrintRecursive(const SimpleJson& json_obj, std::stringstream& ss, int current_indent, int indent_size);
#endif
};

} // namespace utils
} // namespace sdk
} // namespace croupier