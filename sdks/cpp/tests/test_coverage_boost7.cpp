// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

// Seventh coverage boost: locks down ClientConfigLoader::LoadFromJson
// behavior around the header mapping helpers — non-object "headers" values
// are tolerated (the mapping is defensive and skips them), while both the
// flat "headers" object and the nested "auth.headers" object keep loading.
// Production code is untouched.

#include <gtest/gtest.h>

#include <string>

#include "croupier/sdk/config/client_config_loader.h"

namespace croupier::sdk::test {
namespace {

using croupier::sdk::config::ClientConfigLoader;

TEST(ClientConfigLoaderBoost7Test, LoadFromJsonToleratesNonObjectHeaders) {
    ClientConfigLoader loader;
    // "headers": 5 is structurally wrong but every mapping helper is
    // defensive: LoadFromJson must not throw and must yield no headers.
    ClientConfig flat = loader.LoadFromJson(R"({"headers": 5})");
    EXPECT_TRUE(flat.headers.empty());
    ClientConfig nested = loader.LoadFromJson(R"({"auth": {"headers": 7}})");
    EXPECT_TRUE(nested.headers.empty());
    // Scalar roots are likewise tolerated by the defensive mapping.
    ClientConfig array_root = loader.LoadFromJson(R"({"game_id": "g", "headers": []})");
    EXPECT_EQ("g", array_root.game_id);
    EXPECT_TRUE(array_root.headers.empty());
}

TEST(ClientConfigLoaderBoost7Test, LoadFromJsonAcceptsFlatAndNestedHeaders) {
    ClientConfigLoader loader;
    ClientConfig flat = loader.LoadFromJson(R"({"headers": {"X-A": "1"}})");
    EXPECT_EQ("1", flat.headers.at("X-A"));
    ClientConfig nested = loader.LoadFromJson(R"({"auth": {"headers": {"X-B": "2"}}})");
    EXPECT_EQ("2", nested.headers.at("X-B"));
}

}  // namespace
}  // namespace croupier::sdk::test
