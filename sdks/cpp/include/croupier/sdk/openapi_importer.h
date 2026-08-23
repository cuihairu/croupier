// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

#pragma once

#include "croupier/sdk/croupier_client.h"

#include <functional>
#include <map>
#include <optional>
#include <string>
#include <vector>

namespace croupier::sdk::openapi {

/**
 * Controls OpenAPI import behaviour (mirrors the Go SDK's ImportOptions).
 */
struct ImportOptions {
    /// Prefix prepended to every imported resource (e.g. "game").
    std::string resource_prefix;
    /// Prefix prepended to every imported tag.
    std::string tag_prefix;
    /// Keep importing remaining operations when one fails.
    bool continue_on_error = false;
};

/**
 * Resolves a handler for a derived function ID.
 * Return std::nullopt to mark the operation as unhandled.
 */
using HandlerResolver =
    std::function<std::optional<FunctionHandler>(const std::string&)>;

/// Receives each derived registration; return false to reject it.
using RegistrationSink =
    std::function<bool(const FunctionDescriptor&, FunctionHandler)>;

/**
 * Imports an OpenAPI 3 spec, converting every operation into a
 * FunctionDescriptor and handing it to the registration sink (mirrors the Go
 * SDK's RegisterFromOpenAPI).
 *
 * @param sink     receives each descriptor/handler pair
 * @param spec     OpenAPI 3 JSON document
 * @param options  import options
 * @param resolver supplies handlers for derived function IDs
 * @return the list of registered function IDs; throws std::runtime_error on
 *         invalid specs or missing handlers (unless continue_on_error is set)
 */
std::vector<std::string> RegisterFromOpenAPI(const RegistrationSink& sink,
                                             const std::string& spec,
                                             const ImportOptions& options,
                                             const HandlerResolver& resolver);

/**
 * Imports an OpenAPI 3 spec, registering every operation on a CroupierClient.
 */
std::vector<std::string> RegisterFromOpenAPI(CroupierClient& client,
                                             const std::string& spec,
                                             const ImportOptions& options,
                                             const HandlerResolver& resolver);

/**
 * Imports an OpenAPI 3 spec using an explicit handler map (mirrors the Go
 * SDK's RegisterFromOpenAPIWithHandlers).
 */
std::vector<std::string> RegisterFromOpenAPIWithHandlers(
    CroupierClient& client,
    const std::string& spec,
    const ImportOptions& options,
    const std::map<std::string, FunctionHandler>& handlers);

}  // namespace croupier::sdk::openapi
