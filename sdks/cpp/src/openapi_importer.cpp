// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

#include "croupier/sdk/openapi_importer.h"

#include <nlohmann/json.hpp>

#include <algorithm>
#include <cctype>
#include <stdexcept>

namespace croupier::sdk::openapi {
namespace {

constexpr const char* kOperationMethods[] = {
    "get", "put", "post", "delete", "options", "head", "patch", "trace"};

std::string DeriveOperationId(const nlohmann::json& operation, const std::string& path) {
    if (operation.contains("operationId") && operation["operationId"].is_string() &&
        !operation["operationId"].get<std::string>().empty()) {
        return operation["operationId"].get<std::string>();
    }
    if (!path.empty()) {
        std::vector<std::string> segments;
        std::string current;
        for (const char c : path) {
            if (c == '/') {
                if (!current.empty()) segments.push_back(current);
                current.clear();
            } else {
                current += c;
            }
        }
        if (!current.empty()) segments.push_back(current);
        if (!segments.empty()) {
            std::string joined;
            for (size_t i = 0; i < segments.size(); ++i) {
                if (i > 0) joined += '.';
                joined += segments[i];
            }
            return joined;
        }
    }
    return "unknown.function";
}

std::string ToTitleCase(const std::string& value) {
    std::string output;
    bool at_underscore_boundary = true;
    for (const char raw : value) {
        if (raw == '_') {
            output += ' ';
            at_underscore_boundary = true;
            continue;
        }
        const unsigned char c = static_cast<unsigned char>(raw);
        if (at_underscore_boundary && std::isalpha(c)) {
            output += static_cast<char>(std::toupper(c));
            at_underscore_boundary = false;
        } else if (std::isalpha(c)) {
            output += static_cast<char>(std::tolower(c));
        } else {
            output += raw;
            at_underscore_boundary = false;
        }
    }
    return output;
}

std::string DeriveSummary(const nlohmann::json& operation, const std::string& function_id) {
    if (operation.contains("summary") && operation["summary"].is_string() &&
        !operation["summary"].get<std::string>().empty()) {
        return operation["summary"].get<std::string>();
    }
    if (function_id != "unknown.function") {
        return ToTitleCase(function_id);
    }
    return "Unnamed Function";
}

/// Shallow OpenAPI-schema -> JSON-Schema conversion (Go parity).
std::optional<std::string> SchemaToJsonSchema(const nlohmann::json& schema) {
    if (!schema.is_object() || schema.empty()) return std::nullopt;
    nlohmann::json result = nlohmann::json::object();
    if (schema.contains("type") && schema["type"].is_string()) {
        result["type"] = schema["type"];
    }
    if (schema.contains("description") && schema["description"].is_string() &&
        !schema["description"].get<std::string>().empty()) {
        result["description"] = schema["description"];
    }
    if (schema.contains("properties") && schema["properties"].is_object() &&
        !schema["properties"].empty()) {
        nlohmann::json props = nlohmann::json::object();
        for (const auto& [name, prop] : schema["properties"].items()) {
            if (!prop.is_object()) continue;
            nlohmann::json entry = nlohmann::json::object();
            entry["type"] = (prop.contains("type") && prop["type"].is_string())
                                ? prop["type"]
                                : nlohmann::json("object");
            if (prop.contains("description") && prop["description"].is_string() &&
                !prop["description"].get<std::string>().empty()) {
                entry["description"] = prop["description"];
            }
            props[name] = std::move(entry);
        }
        result["properties"] = std::move(props);
    }
    if (schema.contains("required") && schema["required"].is_array() &&
        !schema["required"].empty()) {
        result["required"] = schema["required"];
    }
    if (result.empty()) return std::nullopt;
    return result.dump();
}

std::optional<std::string> JsonContentSchema(const nlohmann::json& holder) {
    if (!holder.is_object() || !holder.contains("content") || !holder["content"].is_object()) {
        return std::nullopt;
    }
    const auto& content = holder["content"];
    if (!content.contains("application/json") || !content["application/json"].is_object()) {
        return std::nullopt;
    }
    const auto& media = content["application/json"];
    if (!media.contains("schema")) return std::nullopt;
    return SchemaToJsonSchema(media["schema"]);
}

std::string ExtractExtension(const nlohmann::json& operation, const std::string& key) {
    if (!operation.contains(key)) return "";
    const auto& value = operation[key];
    if (value.is_string()) return value.get<std::string>();
    if (value.is_boolean()) return value.get<bool>() ? "true" : "false";
    return value.dump();
}

std::string ParseRiskLevel(const std::string& level) {
    std::string normalized;
    normalized.reserve(level.size());
    for (const char c : level) normalized += static_cast<char>(std::tolower(static_cast<unsigned char>(c)));
    if (normalized == "low" || normalized == "safe") return "low";
    if (normalized == "high") return "high";
    if (normalized == "danger" || normalized == "critical") return "danger";
    return "medium";
}

FunctionDescriptor OperationToDescriptor(const std::string& path,
                                         const nlohmann::json& operation,
                                         const ImportOptions& options) {
    FunctionDescriptor descriptor;
    descriptor.id = DeriveOperationId(operation, path);
    descriptor.summary = DeriveSummary(operation, descriptor.id);
    if (operation.contains("description") && operation["description"].is_string()) {
        descriptor.description = operation["description"].get<std::string>();
    }
    if (operation.contains("tags") && operation["tags"].is_array()) {
        for (const auto& tag : operation["tags"]) {
            if (tag.is_string()) descriptor.tags.push_back(tag.get<std::string>());
        }
    }

    std::string resource = ExtractExtension(operation, "x-resource");
    if (!resource.empty()) descriptor.resource = resource;
    std::string operation_name = ExtractExtension(operation, "x-operation");
    if (!operation_name.empty()) descriptor.operation = operation_name;
    std::string permission = ExtractExtension(operation, "x-permission");
    if (!permission.empty()) descriptor.permission = permission;

    if (auto input = JsonContentSchema(operation.contains("requestBody") ? operation["requestBody"] : nlohmann::json::object())) {
        descriptor.input_schema = *input;
    }
    if (operation.contains("responses") && operation["responses"].is_object() &&
        operation["responses"].contains("200")) {
        if (auto output = JsonContentSchema(operation["responses"]["200"])) {
            descriptor.output_schema = *output;
        }
    }

    std::string risk = ExtractExtension(operation, "x-risk");
    descriptor.risk = risk.empty() ? "medium" : ParseRiskLevel(risk);

    if (!options.resource_prefix.empty() && !resource.empty()) {
        descriptor.resource = options.resource_prefix + "." + resource;
    }
    if (!options.tag_prefix.empty()) {
        for (auto& tag : descriptor.tags) tag = options.tag_prefix + tag;
    }
    return descriptor;
}

}  // namespace

std::vector<std::string> RegisterFromOpenAPI(const RegistrationSink& sink,
                                             const std::string& spec,
                                             const ImportOptions& options,
                                             const HandlerResolver& resolver) {
    nlohmann::ordered_json document;
    try {
        document = nlohmann::ordered_json::parse(spec);
    } catch (const nlohmann::json::parse_error& error) {
        throw std::runtime_error(std::string("load OpenAPI spec failed: ") + error.what());
    }
    if (!document.is_object() || !document.contains("paths") || !document["paths"].is_object()) {
        throw std::runtime_error("OpenAPI spec must be an object containing 'paths'");
    }

    std::vector<std::string> registered;
    for (const auto& [path, path_item] : document["paths"].items()) {
        if (!path_item.is_object()) continue;
        for (const char* method : kOperationMethods) {
            if (!path_item.contains(method) || !path_item[method].is_object()) continue;
            const nlohmann::json& operation = path_item[method];

            FunctionDescriptor descriptor = OperationToDescriptor(path, operation, options);
            auto handler = resolver(descriptor.id);
            if (!handler.has_value() || !*handler) {
                if (options.continue_on_error) continue;
                throw std::runtime_error("no handler provided for function: " + descriptor.id);
            }
            if (!sink(descriptor, *handler)) {
                if (options.continue_on_error) continue;
                throw std::runtime_error("register function " + descriptor.id + " failed");
            }
            registered.push_back(descriptor.id);
        }
    }
    return registered;
}

std::vector<std::string> RegisterFromOpenAPI(CroupierClient& client,
                                             const std::string& spec,
                                             const ImportOptions& options,
                                             const HandlerResolver& resolver) {
    return RegisterFromOpenAPI(
        [&client](const FunctionDescriptor& descriptor, FunctionHandler handler) {
            return client.RegisterFunction(descriptor, std::move(handler));
        },
        spec, options, resolver);
}

std::vector<std::string> RegisterFromOpenAPIWithHandlers(
    CroupierClient& client,
    const std::string& spec,
    const ImportOptions& options,
    const std::map<std::string, FunctionHandler>& handlers) {
    HandlerResolver resolver = [&handlers](const std::string& function_id) {
        auto it = handlers.find(function_id);
        if (it == handlers.end()) return std::optional<FunctionHandler>(std::nullopt);
        return std::optional<FunctionHandler>(it->second);
    };
    return RegisterFromOpenAPI(client, spec, options, resolver);
}

}  // namespace croupier::sdk::openapi
