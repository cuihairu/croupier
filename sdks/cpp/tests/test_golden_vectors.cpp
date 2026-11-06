// CR-FIX-05 cross-SDK behavioural contract golden vectors (C++ surface).
// Reads proto/contracts/golden_vectors.json and asserts NewMessage produces the
// exact frame bytes and ParseMessage round-trips them.

#include <gtest/gtest.h>

#include <nlohmann/json.hpp>

#include <cstdint>
#include <filesystem>
#include <fstream>
#include <string>
#include <vector>

#include "croupier/sdk/protocol.h"

namespace croupier {
namespace sdk {
namespace protocol {
namespace test {

namespace {

std::filesystem::path GoldenPath() {
#ifdef CROUPIER_GOLDEN_VECTORS_PATH
    return std::filesystem::path(CROUPIER_GOLDEN_VECTORS_PATH);
#else
    return std::filesystem::path(__FILE__).parent_path().parent_path().parent_path()
        .parent_path() / "proto" / "contracts" / "golden_vectors.json";
#endif
}

uint8_t Nibble(char c) {
    if (c >= '0' && c <= '9') return static_cast<uint8_t>(c - '0');
    if (c >= 'a' && c <= 'f') return static_cast<uint8_t>(c - 'a' + 10);
    if (c >= 'A' && c <= 'F') return static_cast<uint8_t>(c - 'A' + 10);
    throw std::runtime_error("invalid hex digit");
}

std::vector<uint8_t> FromHex(const std::string& hex) {
    if (hex.size() % 2 != 0) throw std::runtime_error("odd hex length");
    std::vector<uint8_t> out(hex.size() / 2);
    for (size_t i = 0; i < out.size(); ++i) {
        out[i] = static_cast<uint8_t>((Nibble(hex[i * 2]) << 4) | Nibble(hex[i * 2 + 1]));
    }
    return out;
}

uint32_t ParseHexId(const std::string& hex) {
    return static_cast<uint32_t>(std::stoul(hex, nullptr, 16));
}

}  // namespace

TEST(GoldenVectorsTest, WireFramesEncodeAndDecode) {
    std::ifstream in(GoldenPath());
    ASSERT_TRUE(in.good()) << "cannot open " << GoldenPath();
    nlohmann::json doc;
    in >> doc;

    for (auto it = doc.at("vectors").begin(); it != doc.at("vectors").end(); ++it) {
        const std::string& name = it.key();
        const auto& vec = it.value();
        uint32_t msg_id = ParseHexId(vec.at("msgId").get<std::string>());
        uint32_t req_id = ParseHexId(vec.at("reqId").get<std::string>());
        auto body = FromHex(vec.at("body").get<std::string>());
        auto want = FromHex(vec.at("frame").get<std::string>());

        auto got = NewMessage(msg_id, req_id, body);
        EXPECT_EQ(want, got) << name << ": frame mismatch";

        ParsedMessage parsed = ParseMessage(want);
        EXPECT_EQ(VERSION_1, parsed.version) << name;
        EXPECT_EQ(msg_id, parsed.msg_id) << name;
        EXPECT_EQ(req_id, parsed.req_id) << name;
        EXPECT_EQ(body, parsed.body) << name;
    }
}

TEST(GoldenVectorsTest, UsesKnownMsgIdsFromContractSurface) {
    std::ifstream in(GoldenPath());
    ASSERT_TRUE(in.good()) << "cannot open " << GoldenPath();
    nlohmann::json doc;
    in >> doc;
    const auto& vectors = doc.at("vectors");

    EXPECT_EQ(MSG_REGISTER_REQUEST,
              ParseHexId(vectors.at("register_request").at("msgId").get<std::string>()));
    EXPECT_EQ(MSG_HEARTBEAT_REQUEST,
              ParseHexId(vectors.at("heartbeat_request").at("msgId").get<std::string>()));
    EXPECT_EQ(MSG_TASK_EVENT,
              ParseHexId(vectors.at("task_event").at("msgId").get<std::string>()));
}

}  // namespace test
}  // namespace protocol
}  // namespace sdk
}  // namespace croupier
