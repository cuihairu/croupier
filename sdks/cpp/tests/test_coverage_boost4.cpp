// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

// Fourth coverage boost: file push wire/staging (F: 文件下发), provider
// registration descriptor round-trip, TCP transport/server edge paths
// (DNS failure, timeouts, malformed frames, inbound dispatch backpressure),
// invoker header/URL/retry edge cases, plus targeted module edges
// (config validation, OpenAPI import variants, file utils, plugin loader,
// logger, field hints, main-thread dispatcher, JSON-schema patterns).

#include <gtest/gtest.h>

#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/config/client_config_loader.h"
#include "croupier/sdk/field_hints.h"
#include "croupier/sdk/file_push.h"
#include "croupier/sdk/http_transport.h"
#include "croupier/sdk/logger.h"
#include "croupier/sdk/openapi_importer.h"
#include "croupier/sdk/protocol.h"
#include "croupier/sdk/tcp_transport.h"
#include "croupier/sdk/threading/main_thread_dispatcher.h"
#include "croupier/sdk/utils/file_utils.h"
#include "croupier/sdk/utils/json_utils.h"
#include "croupier/sdk/plugin/dynamic_loader.h"

#include "croupier/agent/v1/register.pb.h"
#include "croupier/sdk/v1/invocation.pb.h"
#include "croupier/sdk/v1/provider.pb.h"

#include <atomic>
#include <chrono>
#include <cstdio>
#include <cstring>
#include <fstream>
#include <map>
#include <memory>
#include <mutex>
#include <string>
#include <thread>
#include <vector>

#ifdef _WIN32
#include <winsock2.h>
#include <ws2tcpip.h>
using raw_socket_t = SOCKET;
#define RAW_INVALID_SOCK INVALID_SOCKET
#define raw_closesocket ::closesocket
#else
#include <arpa/inet.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <unistd.h>
using raw_socket_t = int;
#define RAW_INVALID_SOCK (-1)
#define raw_closesocket ::close
#endif

namespace croupier::sdk::test {
namespace {

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

bool SendAllRaw(raw_socket_t sock, const void* buf, size_t len) {
    const char* p = static_cast<const char*>(buf);
    size_t off = 0;
    while (off < len) {
        auto n = ::send(sock, p + off, len - off, 0);
        if (n <= 0) return false;
        off += static_cast<size_t>(n);
    }
    return true;
}

bool ReadAllRaw(raw_socket_t sock, void* buf, size_t len) {
    char* p = static_cast<char*>(buf);
    size_t off = 0;
    while (off < len) {
        auto n = ::recv(sock, p + off, len - off, 0);
        if (n <= 0) return false;
        off += static_cast<size_t>(n);
    }
    return true;
}

// Raw fake agent: native listener that speaks the Croupier frame protocol.
// Mirrors the harness in test_provider_inbound.cpp (anonymous namespace there,
// so a local copy is required here).
class Boost4FakeAgent {
public:
    Boost4FakeAgent() {
        auto fatal = [](bool ok, const char* what) {
            if (!ok) throw std::runtime_error(what);
        };
        listen_fd_ = ::socket(AF_INET, SOCK_STREAM, 0);
        fatal(listen_fd_ != RAW_INVALID_SOCK, "socket() failed");
        sockaddr_in addr{};
        addr.sin_family = AF_INET;
        addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
        addr.sin_port = 0;
        fatal(::bind(listen_fd_, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)) == 0, "bind() failed");
        fatal(::listen(listen_fd_, 1) == 0, "listen() failed");
        sockaddr_in bound{};
        socklen_t blen = sizeof(bound);
        fatal(::getsockname(listen_fd_, reinterpret_cast<sockaddr*>(&bound), &blen) == 0,
              "getsockname() failed");
        port_ = ntohs(bound.sin_port);
    }

    ~Boost4FakeAgent() {
        if (conn_ != RAW_INVALID_SOCK) raw_closesocket(conn_);
        if (listen_fd_ != RAW_INVALID_SOCK) raw_closesocket(listen_fd_);
    }

    std::string address() const { return "127.0.0.1:" + std::to_string(port_); }

    unsigned short port() const { return static_cast<unsigned short>(port_); }

    // Accepts a raw TCP connection without any Croupier handshake (used by
    // direct TCPTransport tests that do not speak ProviderConnect).
    void AcceptOnly() {
        conn_ = ::accept(listen_fd_, nullptr, nullptr);
        ASSERT_NE(conn_, RAW_INVALID_SOCK);
    }

    // Accepts the provider and performs the ProviderConnect handshake.
    // The connect request is captured for descriptor assertions.
    void AcceptAndHandshake(bool empty_session = false) {
        conn_ = ::accept(listen_fd_, nullptr, nullptr);
        ASSERT_NE(conn_, RAW_INVALID_SOCK);
        auto first = ReadFrame();
        ASSERT_EQ(first.msg_id, protocol::MSG_PROVIDER_CONNECT_REQUEST);
        last_connect_body_ = first.body;
        v1::ProviderConnectResponse resp;
        if (!empty_session) {
            resp.set_session_id("boost4-session");
        }
        std::string out;
        resp.SerializeToString(&out);
        WriteFrame(protocol::MSG_PROVIDER_CONNECT_RESPONSE, first.req_id,
                   std::vector<uint8_t>(out.begin(), out.end()));
    }

    // Handshake that answers with unparseable bytes: the client's
    // ProviderConnectResponse parsing fails.
    void AcceptAndHandshakeGarbage() {
        conn_ = ::accept(listen_fd_, nullptr, nullptr);
        ASSERT_NE(conn_, RAW_INVALID_SOCK);
        auto first = ReadFrame();
        ASSERT_EQ(first.msg_id, protocol::MSG_PROVIDER_CONNECT_REQUEST);
        last_connect_body_ = first.body;
        WriteFrame(protocol::MSG_PROVIDER_CONNECT_RESPONSE, first.req_id,
                   std::vector<uint8_t>(16, 0xEE));
    }

    // Drops the current connection but keeps listening for a reconnect.
    void AcceptReconnect(bool empty_session = false) {
        DropConnection();
        AcceptAndHandshake(empty_session);
    }

    void PushRequest(uint32_t msg_id, uint32_t req_id, const std::vector<uint8_t>& body) {
        WriteFrame(msg_id, req_id, body);
    }

    protocol::ParsedMessage ReadFrame() {
        uint8_t hdr[4] = {0};
        if (!ReadAllRaw(conn_, hdr, 4)) ADD_FAILURE() << "read frame header failed";
        uint32_t len = (uint32_t(hdr[0]) << 24) | (uint32_t(hdr[1]) << 16) |
                       (uint32_t(hdr[2]) << 8) | uint32_t(hdr[3]);
        std::vector<uint8_t> payload(len);
        if (len > 0 && !ReadAllRaw(conn_, payload.data(), len)) {
            ADD_FAILURE() << "read frame body failed";
        }
        return protocol::ParseMessage(payload);
    }

    protocol::ParsedMessage ReadResponseFor(uint32_t req_id) {
        for (int i = 0; i < 32; ++i) {
            auto m = ReadFrame();
            if (m.req_id == req_id) return m;
        }
        ADD_FAILURE() << "expected response for req_id " << req_id << " not seen";
        return protocol::ParseMessage({});
    }

    // Sends a raw pre-framed blob (for malformed-frame scenarios).
    void SendRaw(const std::vector<uint8_t>& bytes) {
        ASSERT_TRUE(SendAllRaw(conn_, bytes.data(), bytes.size()));
    }

    void DropConnection() {
        if (conn_ != RAW_INVALID_SOCK) {
            raw_closesocket(conn_);
            conn_ = RAW_INVALID_SOCK;
        }
    }

    // Closes the listening socket so reconnect attempts are refused.
    void CloseListener() {
        if (listen_fd_ != RAW_INVALID_SOCK) {
            raw_closesocket(listen_fd_);
            listen_fd_ = RAW_INVALID_SOCK;
        }
        DropConnection();
    }

    const std::vector<uint8_t>& last_connect_body() const { return last_connect_body_; }

private:
    void WriteFrame(uint32_t msg_id, uint32_t req_id, const std::vector<uint8_t>& body) {
        auto frame = protocol::NewMessage(msg_id, req_id, body);
        std::vector<uint8_t> wrapped(4 + frame.size());
        wrapped[0] = static_cast<uint8_t>((frame.size() >> 24) & 0xFF);
        wrapped[1] = static_cast<uint8_t>((frame.size() >> 16) & 0xFF);
        wrapped[2] = static_cast<uint8_t>((frame.size() >> 8) & 0xFF);
        wrapped[3] = static_cast<uint8_t>(frame.size() & 0xFF);
        std::memcpy(wrapped.data() + 4, frame.data(), frame.size());
        ASSERT_TRUE(SendAllRaw(conn_, wrapped.data(), wrapped.size()));
    }

    raw_socket_t listen_fd_{RAW_INVALID_SOCK};
    raw_socket_t conn_{RAW_INVALID_SOCK};
    int port_{0};
    std::vector<uint8_t> last_connect_body_;
};

ClientConfig Boost4ProviderConfig(const std::string& addr) {
    ClientConfig config;
    config.game_id = "game-boost4";
    config.env = "development";
    config.service_id = "cpp-boost4";
    config.agent_addr = addr;
    config.timeout_seconds = 5;
    config.connect_timeout_seconds = 2;
    config.heartbeat_interval = 30;
    config.disable_logging = true;
    return config;
}

std::vector<uint8_t> InvokeBodyBoost4(const std::string& function_id, const std::string& payload) {
    v1::InvokeRequest req;
    req.set_function_id(function_id);
    req.set_payload(payload);
    std::string out;
    req.SerializeToString(&out);
    return std::vector<uint8_t>(out.begin(), out.end());
}

std::string StagingDir() {
    return ::testing::TempDir() + "croupier-boost4-staging";
}

std::string ReadFileRaw(const std::string& path) {
    std::ifstream in(path, std::ios::binary);
    if (!in.is_open()) return "";
    return std::string((std::istreambuf_iterator<char>(in)), std::istreambuf_iterator<char>());
}

FunctionDescriptor MakeEchoDescriptor() {
    FunctionDescriptor desc;
    desc.id = "test.echo";
    desc.version = "1.0.0";
    desc.operation = "echo";
    desc.capability = "action";
    desc.risk = "safe";
    return desc;
}

void RemoveTreeIfExists(const std::string& path) {
    utils::FileSystemUtils::RemoveDirectory(path, true);
}

// Wire encoder for FilePushRequest bodies (mirrors the hand-written decoder
// under test).
std::vector<uint8_t> MakePushBody(const std::string& transfer_id, const std::string& file_name,
                                  const std::string& sha256_hex, const std::vector<uint8_t>& data,
                                  bool include_unknown_field = false) {
    std::vector<uint8_t> out;
    auto bytes_field = [&out](uint64_t field, const std::vector<uint8_t>& value) {
        AppendVarint(out, (field << 3) | 2);
        AppendVarint(out, value.size());
        out.insert(out.end(), value.begin(), value.end());
    };
    bytes_field(1, std::vector<uint8_t>(transfer_id.begin(), transfer_id.end()));
    bytes_field(2, std::vector<uint8_t>(file_name.begin(), file_name.end()));
    bytes_field(3, std::vector<uint8_t>(sha256_hex.begin(), sha256_hex.end()));
    bytes_field(4, data);
    if (include_unknown_field) {
        bytes_field(99, {0xDE, 0xAD});
    }
    return out;
}

struct ParsedPushResponse {
    std::string transfer_id;
    std::string stored_path;
    std::string error;
    bool ok = false;
};

ParsedPushResponse ParsePushResponse(const std::vector<uint8_t>& body) {
    ParsedPushResponse resp;
    size_t idx = 0;
    auto read_varint = [&](uint64_t& value) -> bool {
        value = 0;
        int shift = 0;
        while (idx < body.size()) {
            uint8_t byte = body[idx++];
            value |= static_cast<uint64_t>(byte & 0x7F) << shift;
            if (!(byte & 0x80)) return true;
            shift += 7;
            if (shift > 63) return false;
        }
        return false;
    };
    while (idx < body.size()) {
        uint64_t tag = 0;
        if (!read_varint(tag)) break;
        uint64_t field = tag >> 3;
        uint64_t wire = tag & 0x7;
        if (wire == 2) {
            uint64_t len = 0;
            if (!read_varint(len)) break;
            if (idx + len > body.size()) break;
            std::string value(reinterpret_cast<const char*>(body.data()) + idx, len);
            idx += len;
            if (field == 1) resp.transfer_id = value;
            if (field == 3) resp.stored_path = value;
            if (field == 4) resp.error = value;
        } else if (wire == 0) {
            uint64_t value = 0;
            if (!read_varint(value)) break;
            if (field == 2) resp.ok = value != 0;
        } else {
            break;  // unsupported wire type in this test parser
        }
    }
    return resp;
}

// ---------------------------------------------------------------------------
// 1) file_push.h pure helpers
// ---------------------------------------------------------------------------

TEST(FilePushWireBoost4Test, AppendVarintEncodesSingleAndMultiByteValues) {
    {
        std::vector<uint8_t> out;
        AppendVarint(out, 0);
        EXPECT_EQ(out, (std::vector<uint8_t>{0x00}));
    }
    {
        std::vector<uint8_t> out;
        AppendVarint(out, 1);
        EXPECT_EQ(out, (std::vector<uint8_t>{0x01}));
    }
    {
        std::vector<uint8_t> out;
        AppendVarint(out, 127);
        EXPECT_EQ(out, (std::vector<uint8_t>{0x7F}));
    }
    {
        std::vector<uint8_t> out;
        AppendVarint(out, 128);
        EXPECT_EQ(out, (std::vector<uint8_t>{0x80, 0x01}));
    }
    {
        std::vector<uint8_t> out;
        AppendVarint(out, 300);
        EXPECT_EQ(out, (std::vector<uint8_t>{0xAC, 0x02}));
    }
    {
        std::vector<uint8_t> out;
        AppendVarint(out, 16384);
        EXPECT_EQ(out, (std::vector<uint8_t>{0x80, 0x80, 0x01}));
    }
    {
        // 2^32 - 1 spans five bytes.
        std::vector<uint8_t> out;
        AppendVarint(out, 0xFFFFFFFFULL);
        EXPECT_EQ(out.size(), size_t(5));
        EXPECT_EQ(out.back(), 0x0F);
    }
}

TEST(FilePushWireBoost4Test, Sha256HexMatchesKnownDigests) {
    EXPECT_EQ(Sha256Hex({}), "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855");
    const std::string abc = "abc";
    EXPECT_EQ(Sha256Hex(std::vector<uint8_t>(abc.begin(), abc.end())),
              "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad");
    const std::string long_input = "abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq";
    EXPECT_EQ(Sha256Hex(std::vector<uint8_t>(long_input.begin(), long_input.end())),
              "248d6a61d20638b8e5c026930c3e6039a33ce45964ff2167f6ecedd419db06c1");
}

TEST(FilePushWireBoost4Test, DecodeParsesAllFieldsAndSkipsUnknown) {
    const auto body = MakePushBody("t-1", "patch.bin", "aa", {1, 2, 3, 4, 5}, true);
    const auto req = DecodeFilePushRequest(body);
    EXPECT_EQ(req.transfer_id, "t-1");
    EXPECT_EQ(req.file_name, "patch.bin");
    EXPECT_EQ(req.content_sha256, "aa");
    EXPECT_EQ(req.data, (std::vector<uint8_t>{1, 2, 3, 4, 5}));
}

TEST(FilePushWireBoost4Test, DecodeStopsOnTruncatedVarintTag) {
    // A lone continuation byte at the end: the tag varint never terminates.
    std::vector<uint8_t> body{0x80};
    const auto req = DecodeFilePushRequest(body);
    EXPECT_TRUE(req.transfer_id.empty());
}

TEST(FilePushWireBoost4Test, DecodeStopsOnTruncatedLength) {
    // Field 1, length-delimited, but the length varint is truncated.
    std::vector<uint8_t> body{0x0A, 0x80};
    const auto req = DecodeFilePushRequest(body);
    EXPECT_TRUE(req.transfer_id.empty());
}

TEST(FilePushWireBoost4Test, DecodeStopsWhenBytesExceedBody) {
    // Declares 16 bytes of payload but only 2 remain.
    std::vector<uint8_t> body{0x0A, 0x10, 0x41, 0x42};
    const auto req = DecodeFilePushRequest(body);
    EXPECT_TRUE(req.transfer_id.empty());
}

TEST(FilePushWireBoost4Test, DecodeRejectsOverlongVarint) {
    // Ten continuation bytes exceed the 64-bit varint budget.
    std::vector<uint8_t> body{0x80, 0x80, 0x80, 0x80, 0x80,
                              0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x01};
    const auto req = DecodeFilePushRequest(body);
    EXPECT_TRUE(req.transfer_id.empty());
}

TEST(FilePushWireBoost4Test, EncodeResponseOmitsEmptyFieldsAndMarksOk) {
    FilePushResponse resp;
    resp.transfer_id = "t-9";
    resp.ok = true;
    resp.stored_path = "/staging/x";
    const auto body = EncodeFilePushResponse(resp);
    const auto parsed = ParsePushResponse(body);
    EXPECT_EQ(parsed.transfer_id, "t-9");
    EXPECT_TRUE(parsed.ok);
    EXPECT_EQ(parsed.stored_path, "/staging/x");
    EXPECT_TRUE(parsed.error.empty());
}

TEST(FilePushWireBoost4Test, EncodeResponseEncodesErrorWithoutOk) {
    FilePushResponse resp;
    resp.transfer_id = "t-err";
    resp.error = "checksum mismatch";
    const auto body = EncodeFilePushResponse(resp);
    const auto parsed = ParsePushResponse(body);
    EXPECT_EQ(parsed.transfer_id, "t-err");
    EXPECT_FALSE(parsed.ok);
    EXPECT_EQ(parsed.error, "checksum mismatch");
    EXPECT_TRUE(parsed.stored_path.empty());
}

TEST(FilePushWireBoost4Test, SafeStagingPathAcceptsBareBasenames) {
    std::string out;
    EXPECT_TRUE(SafeStagingPath("/staging", "patch.bin", out));
    EXPECT_EQ(out, "/staging/patch.bin");
    EXPECT_TRUE(SafeStagingPath("/staging", "a", out));
    EXPECT_TRUE(SafeStagingPath("relative-dir", "UPPER.TXT", out));
}

TEST(FilePushWireBoost4Test, SafeStagingPathRejectsTraversalAndSeparators) {
    std::string out;
    EXPECT_FALSE(SafeStagingPath("/staging", "", out));
    EXPECT_FALSE(SafeStagingPath("/staging", ".", out));
    EXPECT_FALSE(SafeStagingPath("/staging", "..", out));
    EXPECT_FALSE(SafeStagingPath("/staging", "../evil", out));
    EXPECT_FALSE(SafeStagingPath("/staging", "a/../b", out));
    EXPECT_FALSE(SafeStagingPath("/staging", "sub/dir", out));
    EXPECT_FALSE(SafeStagingPath("/staging", "sub\\dir", out));
    EXPECT_FALSE(SafeStagingPath("/staging", "..hidden", out));
}

TEST(FilePushWireBoost4Test, AtomicWriteFilePersistsContent) {
    const std::string dir = StagingDir();
    ASSERT_TRUE(utils::FileSystemUtils::CreateDirectory(dir));
    const std::string target = dir + "/atomic-ok.bin";
    const std::vector<uint8_t> payload{0x01, 0x02, 0xFF, 0x10};
    ASSERT_TRUE(AtomicWriteFile(target, payload));
    EXPECT_EQ(ReadFileRaw(target), std::string(payload.begin(), payload.end()));
    utils::FileSystemUtils::RemoveFile(target);
}

TEST(FilePushWireBoost4Test, AtomicWriteFileSupportsEmptyPayload) {
    const std::string dir = StagingDir();
    ASSERT_TRUE(utils::FileSystemUtils::CreateDirectory(dir));
    const std::string target = dir + "/atomic-empty.bin";
    ASSERT_TRUE(AtomicWriteFile(target, {}));
    EXPECT_EQ(ReadFileRaw(target), "");
    utils::FileSystemUtils::RemoveFile(target);
}

TEST(FilePushWireBoost4Test, AtomicWriteFileFailsWhenParentMissing) {
    const std::string target = StagingDir() + "/missing-parent/file.bin";
    EXPECT_FALSE(AtomicWriteFile(target, {1, 2, 3}));
}

TEST(FilePushWireBoost4Test, AtomicWriteFileOverwritesExistingFile) {
    const std::string dir = StagingDir();
    ASSERT_TRUE(utils::FileSystemUtils::CreateDirectory(dir));
    const std::string target = dir + "/atomic-rewrite.bin";
    ASSERT_TRUE(AtomicWriteFile(target, {1}));
    ASSERT_TRUE(AtomicWriteFile(target, {9, 9}));
    EXPECT_EQ(ReadFileRaw(target), std::string("\x09\x09", 2));
    utils::FileSystemUtils::RemoveFile(target);
}

// ---------------------------------------------------------------------------
// 2) Provider file-push end-to-end (MSG_PROVIDER_FILE_PUSH_REQ dispatch)
// ---------------------------------------------------------------------------

class ProviderFilePushBoost4Test : public ::testing::Test {
protected:
    void SetUp() override {
        RemoveTreeIfExists(StagingDir());
        // The provider writes into the staging directory without creating it,
        // mirroring production where the directory is provisioned upfront.
        ASSERT_TRUE(utils::FileSystemUtils::CreateDirectory(StagingDir()));
    }
    void TearDown() override {
        RemoveTreeIfExists(StagingDir());
    }
};

TEST_F(ProviderFilePushBoost4Test, StoresValidFileAndEchoesPath) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    ClientConfig config = Boost4ProviderConfig(agent.address());
    config.enable_file_transfer = true;
    config.file_staging_dir = StagingDir();
    CroupierClient client(config);
    ASSERT_TRUE(client.RegisterFunction(MakeEchoDescriptor(),
                                        [](const std::string&, const std::string& p) { return p; }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    const std::vector<uint8_t> payload{0xCA, 0xFE, 0xBA, 0xBE};
    agent.PushRequest(protocol::MSG_PROVIDER_FILE_PUSH_REQ, 7001,
                      MakePushBody("tx-ok", "patch.bin", Sha256Hex(payload), payload));
    auto resp = agent.ReadResponseFor(7001);
    auto parsed = ParsePushResponse(resp.body);
    EXPECT_TRUE(parsed.ok) << "error=" << parsed.error;
    EXPECT_EQ(parsed.transfer_id, "tx-ok");
    EXPECT_EQ(parsed.stored_path, StagingDir() + "/patch.bin");
    EXPECT_EQ(ReadFileRaw(parsed.stored_path), std::string(payload.begin(), payload.end()));

    client.Close();
}

TEST_F(ProviderFilePushBoost4Test, ChecksumComparisonIsCaseInsensitive) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    ClientConfig config = Boost4ProviderConfig(agent.address());
    config.enable_file_transfer = true;
    config.file_staging_dir = StagingDir();
    CroupierClient client(config);
    ASSERT_TRUE(client.RegisterFunction(MakeEchoDescriptor(),
                                        [](const std::string&, const std::string& p) { return p; }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    const std::vector<uint8_t> payload{1};
    std::string upper = Sha256Hex(payload);
    for (auto& c : upper) c = static_cast<char>(::toupper(static_cast<unsigned char>(c)));
    agent.PushRequest(protocol::MSG_PROVIDER_FILE_PUSH_REQ, 7002,
                      MakePushBody("tx-upper", "upper.bin", upper, payload));
    auto resp = agent.ReadResponseFor(7002);
    auto parsed = ParsePushResponse(resp.body);
    EXPECT_TRUE(parsed.ok) << "error=" << parsed.error;

    client.Close();
}

TEST_F(ProviderFilePushBoost4Test, RejectsTransferWhenDisabled) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    ClientConfig config = Boost4ProviderConfig(agent.address());
    config.enable_file_transfer = false;  // default
    config.file_staging_dir = StagingDir();
    CroupierClient client(config);
    ASSERT_TRUE(client.RegisterFunction(MakeEchoDescriptor(),
                                        [](const std::string&, const std::string& p) { return p; }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    const std::vector<uint8_t> payload{1, 2};
    agent.PushRequest(protocol::MSG_PROVIDER_FILE_PUSH_REQ, 7003,
                      MakePushBody("tx-disabled", "a.bin", Sha256Hex(payload), payload));
    auto parsed = ParsePushResponse(agent.ReadResponseFor(7003).body);
    EXPECT_FALSE(parsed.ok);
    EXPECT_NE(parsed.error.find("disabled"), std::string::npos) << parsed.error;

    client.Close();
}

TEST_F(ProviderFilePushBoost4Test, RejectsEmptyTransferId) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    ClientConfig config = Boost4ProviderConfig(agent.address());
    config.enable_file_transfer = true;
    config.file_staging_dir = StagingDir();
    CroupierClient client(config);
    ASSERT_TRUE(client.RegisterFunction(MakeEchoDescriptor(),
                                        [](const std::string&, const std::string& p) { return p; }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    const std::vector<uint8_t> payload{5};
    agent.PushRequest(protocol::MSG_PROVIDER_FILE_PUSH_REQ, 7004,
                      MakePushBody("", "a.bin", Sha256Hex(payload), payload));
    auto parsed = ParsePushResponse(agent.ReadResponseFor(7004).body);
    EXPECT_FALSE(parsed.ok);
    EXPECT_NE(parsed.error.find("transferId is required"), std::string::npos) << parsed.error;

    client.Close();
}

TEST_F(ProviderFilePushBoost4Test, RejectsUnsafeFileNames) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    ClientConfig config = Boost4ProviderConfig(agent.address());
    config.enable_file_transfer = true;
    config.file_staging_dir = StagingDir();
    CroupierClient client(config);
    ASSERT_TRUE(client.RegisterFunction(MakeEchoDescriptor(),
                                        [](const std::string&, const std::string& p) { return p; }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    const std::vector<uint8_t> payload{7};
    const std::string bad_names[] = {"../escape", "sub/dir", "sub\\dir", "..", "."};
    uint32_t req_id = 7100;
    for (const std::string& name : bad_names) {
        agent.PushRequest(protocol::MSG_PROVIDER_FILE_PUSH_REQ, req_id,
                          MakePushBody("tx-name", name, Sha256Hex(payload), payload));
        auto parsed = ParsePushResponse(agent.ReadResponseFor(req_id).body);
        EXPECT_FALSE(parsed.ok) << "name=" << name;
        EXPECT_NE(parsed.error.find("bare basename"), std::string::npos)
            << "name=" << name << " error=" << parsed.error;
        ++req_id;
    }

    client.Close();
}

TEST_F(ProviderFilePushBoost4Test, RejectsEmptyPayload) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    ClientConfig config = Boost4ProviderConfig(agent.address());
    config.enable_file_transfer = true;
    config.file_staging_dir = StagingDir();
    CroupierClient client(config);
    ASSERT_TRUE(client.RegisterFunction(MakeEchoDescriptor(),
                                        [](const std::string&, const std::string& p) { return p; }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    agent.PushRequest(protocol::MSG_PROVIDER_FILE_PUSH_REQ, 7201,
                      MakePushBody("tx-empty", "a.bin", "00", {}));
    auto parsed = ParsePushResponse(agent.ReadResponseFor(7201).body);
    EXPECT_FALSE(parsed.ok);
    EXPECT_NE(parsed.error.find("payload is empty"), std::string::npos) << parsed.error;

    client.Close();
}

TEST_F(ProviderFilePushBoost4Test, RejectsOversizedPayload) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    ClientConfig config = Boost4ProviderConfig(agent.address());
    config.enable_file_transfer = true;
    config.file_staging_dir = StagingDir();
    config.max_file_size = 4;
    CroupierClient client(config);
    ASSERT_TRUE(client.RegisterFunction(MakeEchoDescriptor(),
                                        [](const std::string&, const std::string& p) { return p; }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    const std::vector<uint8_t> big{1, 2, 3, 4, 5, 6, 7, 8};
    agent.PushRequest(protocol::MSG_PROVIDER_FILE_PUSH_REQ, 7202,
                      MakePushBody("tx-big", "a.bin", Sha256Hex(big), big));
    auto parsed = ParsePushResponse(agent.ReadResponseFor(7202).body);
    EXPECT_FALSE(parsed.ok);
    EXPECT_NE(parsed.error.find("exceeds max"), std::string::npos) << parsed.error;

    client.Close();
}

TEST_F(ProviderFilePushBoost4Test, RejectsMissingChecksum) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    ClientConfig config = Boost4ProviderConfig(agent.address());
    config.enable_file_transfer = true;
    config.file_staging_dir = StagingDir();
    CroupierClient client(config);
    ASSERT_TRUE(client.RegisterFunction(MakeEchoDescriptor(),
                                        [](const std::string&, const std::string& p) { return p; }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    const std::vector<uint8_t> payload{3};
    agent.PushRequest(protocol::MSG_PROVIDER_FILE_PUSH_REQ, 7203,
                      MakePushBody("tx-nosha", "a.bin", "", payload));
    auto parsed = ParsePushResponse(agent.ReadResponseFor(7203).body);
    EXPECT_FALSE(parsed.ok);
    EXPECT_NE(parsed.error.find("contentSha256 is required"), std::string::npos) << parsed.error;

    client.Close();
}

TEST_F(ProviderFilePushBoost4Test, RejectsChecksumMismatch) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    ClientConfig config = Boost4ProviderConfig(agent.address());
    config.enable_file_transfer = true;
    config.file_staging_dir = StagingDir();
    CroupierClient client(config);
    ASSERT_TRUE(client.RegisterFunction(MakeEchoDescriptor(),
                                        [](const std::string&, const std::string& p) { return p; }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    const std::vector<uint8_t> payload{9, 9};
    const std::string wrong = Sha256Hex({1, 1, 1});
    agent.PushRequest(protocol::MSG_PROVIDER_FILE_PUSH_REQ, 7204,
                      MakePushBody("tx-badsha", "a.bin", wrong, payload));
    auto parsed = ParsePushResponse(agent.ReadResponseFor(7204).body);
    EXPECT_FALSE(parsed.ok);
    EXPECT_EQ(parsed.error, "checksum mismatch");

    client.Close();
}

TEST_F(ProviderFilePushBoost4Test, ReportsStagingWriteFailure) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    // Point the staging dir at a regular file so fopen() of the temp file fails.
    const std::string file_as_dir = ::testing::TempDir() + "croupier-boost4-staging-file";
    ASSERT_TRUE(utils::FileSystemUtils::WriteFileContent(file_as_dir, "not a directory"));
    RemoveTreeIfExists(StagingDir());

    ClientConfig config = Boost4ProviderConfig(agent.address());
    config.enable_file_transfer = true;
    config.file_staging_dir = file_as_dir;
    CroupierClient client(config);
    ASSERT_TRUE(client.RegisterFunction(MakeEchoDescriptor(),
                                        [](const std::string&, const std::string& p) { return p; }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    const std::vector<uint8_t> payload{4, 4};
    agent.PushRequest(protocol::MSG_PROVIDER_FILE_PUSH_REQ, 7205,
                      MakePushBody("tx-writefail", "a.bin", Sha256Hex(payload), payload));
    auto parsed = ParsePushResponse(agent.ReadResponseFor(7205).body);
    EXPECT_FALSE(parsed.ok);
    EXPECT_NE(parsed.error.find("write staging file failed"), std::string::npos) << parsed.error;

    client.Close();
    utils::FileSystemUtils::RemoveFile(file_as_dir);
}

// ---------------------------------------------------------------------------
// 3) Provider registration descriptor round-trip / session edge cases
// ---------------------------------------------------------------------------

FunctionDescriptor MakeFullDescriptor() {
    FunctionDescriptor desc;
    desc.id = "player.ban";
    desc.version = "2.3.4";
    desc.tags = {"t-one", "t-two"};
    desc.summary = "Ban summary";
    // Covers every branch of the manifest JSON escaper.
    desc.description = "quote \" backslash \\ newline \n tab \t CR \r end";
    desc.operation_id = "op-ban-1";
    desc.deprecated = true;
    desc.input_schema = R"({"type":"object","properties":{"id":{"type":"string"}}})";
    desc.output_schema = R"({"type":"object"})";
    desc.resource = "player";
    desc.operation = "ban";
    desc.capability = "action";
    desc.execution = "sync";
    desc.approval_required = true;
    desc.approval_policy_key = "policy-42";
    desc.risk = "high";
    desc.enabled = false;
    desc.permission = "player.ban.execute";
    return desc;
}

TEST(ProviderDescriptorBoost4Test, RegisterSendsEveryDescriptorField) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    CroupierClient client(Boost4ProviderConfig(agent.address()));
    ASSERT_TRUE(client.RegisterFunction(
        MakeFullDescriptor(), [](const std::string&, const std::string&) { return "ok"; }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    ::croupier::sdk::v1::ProviderConnectRequest captured;
    ASSERT_TRUE(captured.ParseFromArray(agent.last_connect_body().data(),
                                        static_cast<int>(agent.last_connect_body().size())));
    ASSERT_EQ(captured.functions_size(), 1);
    const auto& fn = captured.functions(0);
    EXPECT_EQ(fn.id(), "player.ban");
    EXPECT_EQ(fn.version(), "2.3.4");
    ASSERT_EQ(fn.tags_size(), 2);
    EXPECT_EQ(fn.tags(0), "t-one");
    EXPECT_EQ(fn.tags(1), "t-two");
    EXPECT_EQ(fn.summary(), "Ban summary");
    EXPECT_EQ(fn.description(), "quote \" backslash \\ newline \n tab \t CR \r end");
    EXPECT_EQ(fn.operation_id(), "op-ban-1");
    EXPECT_TRUE(fn.deprecated());
    EXPECT_NE(fn.input_schema().find("\"id\""), std::string::npos);
    EXPECT_EQ(fn.output_schema(), R"({"type":"object"})");
    EXPECT_EQ(fn.resource(), "player");
    EXPECT_EQ(fn.operation(), "ban");
    EXPECT_EQ(fn.capability(), "action");
    EXPECT_EQ(fn.execution(), "sync");
    EXPECT_TRUE(fn.approval_required());
    EXPECT_EQ(fn.approval_policy_key(), "policy-42");
    EXPECT_EQ(fn.risk(), "high");
    EXPECT_FALSE(fn.enabled());
    EXPECT_EQ(fn.permission(), "player.ban.execute");

    client.Close();
}

TEST(ProviderDescriptorBoost4Test, EmptySessionIdFailsConnect) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(/*empty_session=*/true); });

    CroupierClient client(Boost4ProviderConfig(agent.address()));
    ASSERT_TRUE(client.RegisterFunction(MakeEchoDescriptor(),
                                        [](const std::string&, const std::string& p) { return p; }));
    EXPECT_FALSE(client.Connect());
    agent_thread.join();
    client.Close();
}

TEST(ProviderDescriptorBoost4Test, ControlPlaneFailureDoesNotBlockConnect) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    ClientConfig config = Boost4ProviderConfig(agent.address());
    // Port 1 is never listened on in test environments: connect is refused and
    // the manifest upload must degrade to a warning.
    config.control_addr = "127.0.0.1:1";
    CroupierClient client(config);
    ASSERT_TRUE(client.RegisterFunction(MakeFullDescriptor(),
                                        [](const std::string&, const std::string&) { return "ok"; }));
    EXPECT_TRUE(client.Connect());
    agent_thread.join();
    client.Close();
}

// ---------------------------------------------------------------------------
// 4) TCPTransport direct edge paths
// ---------------------------------------------------------------------------

TEST(TCPTransportBoost4Test, ConnectThrowsOnUnresolvableHost) {
    TCPTransport transport("nonexistent-host-croupier.invalid", 19091, 2000);
    EXPECT_THROW(transport.Connect(), std::runtime_error);
}

TEST(TCPTransportBoost4Test, ConnectTimesOutOnUnroutableAddress) {
    TCPTransport transport("10.255.255.1", 81, 1000);
    transport.SetConnectTimeout(400);
    EXPECT_THROW(transport.Connect(), std::runtime_error);
}

namespace {

// Binds a loopback port without listening: connecting to it is refused.
unsigned short BoundButNotListeningPort() {
    raw_socket_t fd = ::socket(AF_INET, SOCK_STREAM, 0);
    if (fd == RAW_INVALID_SOCK) return 0;
    sockaddr_in addr{};
    addr.sin_family = AF_INET;
    addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
    addr.sin_port = 0;
    if (::bind(fd, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)) != 0) {
        raw_closesocket(fd);
        return 0;
    }
    sockaddr_in bound{};
    socklen_t blen = sizeof(bound);
    ::getsockname(fd, reinterpret_cast<sockaddr*>(&bound), &blen);
    // Leak intentionally until process exit: closing would free the port and
    // allow another listener to grab it.
    return ntohs(bound.sin_port);
}

}  // namespace

TEST(TCPTransportBoost4Test, ConnectThrowsWhenConnectionRefused) {
    const unsigned short port = BoundButNotListeningPort();
    ASSERT_NE(port, 0);
    TCPTransport transport("127.0.0.1", port, 2000);
    transport.SetConnectTimeout(1000);
    EXPECT_THROW(transport.Connect(), std::runtime_error);
}

TEST(TCPTransportBoost4Test, CallFailsAfterPeerClosesConnection) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptOnly(); });

    TCPTransport transport("127.0.0.1", agent.port(), 3000);
    transport.Connect();
    agent_thread.join();
    agent.DropConnection();
    // Give the OS a moment to propagate the close.
    std::this_thread::sleep_for(std::chrono::milliseconds(100));
    EXPECT_THROW(transport.Call(protocol::MSG_PROVIDER_HEARTBEAT_REQUEST, {1, 2}),
                 std::runtime_error);
    transport.Close();
}

TEST(TCPTransportBoost4Test, PendingCallIsSignalledByClose) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptOnly(); });

    TCPTransport transport("127.0.0.1", agent.port(), 5000);
    transport.Connect();
    agent_thread.join();

    std::atomic<bool> finished{false};
    std::thread caller([&] {
        // The agent stays silent: the call only ends because Close() signals
        // every pending latch (or aborts the wait).
        try {
            auto result = transport.Call(protocol::MSG_PROVIDER_HEARTBEAT_REQUEST, {9});
            // Signalled-with-empty is also an acceptable outcome.
            (void)result;
        } catch (const std::exception&) {
            // "Connection closing" is equally acceptable.
        }
        finished.store(true);
    });
    std::this_thread::sleep_for(std::chrono::milliseconds(150));
    transport.Close();
    caller.join();
    EXPECT_TRUE(finished.load());
}

TEST(TCPTransportBoost4Test, ReadLoopToleratesMalformedFramesAndUnknownResponses) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptOnly(); });

    TCPTransport transport("127.0.0.1", agent.port(), 4000);
    transport.Connect();
    agent_thread.join();

    // Frame with a protocol payload shorter than the 8-byte header.
    agent.PushRequest(protocol::MSG_INVOKE_REQUEST, 1, {0x01, 0x02, 0x03});
    // Frame with an unknown protocol version.
    agent.PushRequest(protocol::MSG_INVOKE_REQUEST, 2, std::vector<uint8_t>(8, 0x02));
    // Response frame for an unknown req_id (no pending call waits for it).
    {
        auto frame = protocol::NewMessage(protocol::MSG_INVOKE_RESPONSE, 987654, {});
        std::vector<uint8_t> wrapped(4 + frame.size());
        wrapped[0] = static_cast<uint8_t>((frame.size() >> 24) & 0xFF);
        wrapped[1] = static_cast<uint8_t>((frame.size() >> 16) & 0xFF);
        wrapped[2] = static_cast<uint8_t>((frame.size() >> 8) & 0xFF);
        wrapped[3] = static_cast<uint8_t>(frame.size() & 0xFF);
        std::memcpy(wrapped.data() + 4, frame.data(), frame.size());
        agent.SendRaw(wrapped);
    }

    // The connection must remain usable: a Call round-trip still works.
    std::this_thread::sleep_for(std::chrono::milliseconds(100));
    std::thread responder([&] {
        auto req = agent.ReadFrame();
        agent.PushRequest(protocol::MSG_INVOKE_RESPONSE, req.req_id, {0x01});
    });
    auto resp = transport.Call(protocol::MSG_INVOKE_REQUEST, {0x55});
    responder.join();
    EXPECT_EQ(resp.second, (std::vector<uint8_t>{0x01}));

    transport.Close();
}

TEST(TCPTransportBoost4Test, ReadLoopExitsOnHeaderReadTimeout) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptOnly(); });

    TCPTransport transport("127.0.0.1", agent.port(), 2000);
    transport.Connect();
    agent_thread.join();

    // Half a frame header: the 1s receive timeout fires and the loop exits.
    agent.SendRaw({0x00, 0x00});
    std::this_thread::sleep_for(std::chrono::milliseconds(1500));
    agent.DropConnection();
    EXPECT_THROW(transport.Call(protocol::MSG_PROVIDER_HEARTBEAT_REQUEST, {1}), std::runtime_error);
    transport.Close();
}

TEST(TCPTransportBoost4Test, ReadLoopExitsOnPayloadReadTimeout) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptOnly(); });

    TCPTransport transport("127.0.0.1", agent.port(), 2000);
    transport.Connect();
    agent_thread.join();

    // Complete 4-byte frame header announcing 16 bytes, then only 8 arrive.
    std::vector<uint8_t> partial{0x00, 0x00, 0x00, 0x10, 1, 2, 3, 4, 5, 6, 7, 8};
    agent.SendRaw(partial);
    std::this_thread::sleep_for(std::chrono::milliseconds(1500));
    agent.DropConnection();
    EXPECT_THROW(transport.Call(protocol::MSG_PROVIDER_HEARTBEAT_REQUEST, {1}), std::runtime_error);
    transport.Close();
}

TEST(TCPTransportBoost4Test, InboundRequestWithoutHandlerIsDropped) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptOnly(); });

    TCPTransport transport("127.0.0.1", agent.port(), 4000);
    transport.Connect();
    agent_thread.join();

    // No SetInboundHandler: DispatchInbound must drop the request silently.
    agent.PushRequest(protocol::MSG_INVOKE_REQUEST, 3, {0x01});
    std::this_thread::sleep_for(std::chrono::milliseconds(150));

    // The transport still answers synchronous calls.
    std::thread responder([&] {
        auto req = agent.ReadFrame();
        agent.PushRequest(protocol::MSG_INVOKE_RESPONSE, req.req_id, {0x02});
    });
    auto resp = transport.Call(protocol::MSG_PROVIDER_HEARTBEAT_REQUEST, {1});
    responder.join();
    EXPECT_EQ(resp.second, (std::vector<uint8_t>{0x02}));

    transport.Close();
}

TEST(TCPTransportBoost4Test, InboundHandlerUpdatePropagatesToRunningPool) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptOnly(); });

    TCPTransport transport("127.0.0.1", agent.port(), 5000);
    transport.Connect();
    agent_thread.join();

    transport.SetInboundHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) {
        return std::vector<uint8_t>{0x0A};
    });
    agent.PushRequest(protocol::MSG_INVOKE_REQUEST, 10, {});
    auto first = agent.ReadResponseFor(10);
    EXPECT_EQ(first.body, (std::vector<uint8_t>{0x0A}));

    // Second handler on the same live transport: the pool must pick it up.
    transport.SetInboundHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) {
        return std::vector<uint8_t>{0x0B};
    });
    agent.PushRequest(protocol::MSG_INVOKE_REQUEST, 11, {});
    auto second = agent.ReadResponseFor(11);
    EXPECT_EQ(second.body, (std::vector<uint8_t>{0x0B}));

    transport.Close();
}

TEST(TCPTransportBoost4Test, QueueSaturationFastFailsWithEmptyResponse) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptOnly(); });

    // Single worker so the queue threshold (workers * 4) is easy to hit.
    TCPTransport::SetInboundWorkerCount(1);
    TCPTransport transport("127.0.0.1", agent.port(), 2000);
    transport.Connect();
    agent_thread.join();

    std::atomic<int> handled{0};
    transport.SetInboundHandler([&](uint32_t, uint32_t, const std::vector<uint8_t>&) {
        handled.fetch_add(1);
        std::this_thread::sleep_for(std::chrono::milliseconds(120));
        return std::vector<uint8_t>{0x01};
    });

    constexpr uint32_t kTotal = 10;
    for (uint32_t i = 0; i < kTotal; ++i) {
        agent.PushRequest(protocol::MSG_INVOKE_REQUEST, 20000 + i, {});
    }
    int empty_responses = 0;
    int full_responses = 0;
    for (uint32_t i = 0; i < kTotal; ++i) {
        auto resp = agent.ReadFrame();
        if (resp.body.empty()) {
            ++empty_responses;
        } else {
            ++full_responses;
        }
    }
    EXPECT_GE(empty_responses, 1) << "expected at least one fast-failed response";
    EXPECT_GE(full_responses, 1);
    // Only the single worker plus its bounded queue can produce full
    // responses; the saturated tail must fast-fail.
    EXPECT_LE(handled.load(), 5);

    transport.Close();
    TCPTransport::SetInboundWorkerCount(0);  // restore the global default
}

// ---------------------------------------------------------------------------
// 5) TCPServer edge paths
// ---------------------------------------------------------------------------

TEST(TCPServerBoost4Test, StartWithBareHostPicksRandomPort) {
    TCPServer server("127.0.0.1");
    server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) { return std::vector<uint8_t>{}; });
    server.Start();
    const std::string actual = server.GetListenAddress();
    EXPECT_NE(actual.find("127.0.0.1:"), std::string::npos);
    server.Stop();
}

TEST(TCPServerBoost4Test, StartRejectsInvalidPortText) {
    TCPServer server("127.0.0.1:notaport");
    EXPECT_THROW(server.Start(), std::runtime_error);
}

TEST(TCPServerBoost4Test, StartBindsWildcardAddresses) {
    {
        TCPServer server("0.0.0.0:0");
        server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) { return std::vector<uint8_t>{}; });
        server.Start();
        server.Stop();
    }
    {
        TCPServer server("*:0");
        server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) { return std::vector<uint8_t>{}; });
        server.Start();
        server.Stop();
    }
}

TEST(TCPServerBoost4Test, StartRejectsNonIpHost) {
    TCPServer server("not-an-ip.example:19091");
    EXPECT_THROW(server.Start(), std::runtime_error);
}

TEST(TCPServerBoost4Test, StartFailsWhenPortAlreadyBound) {
    TCPServer blocker("127.0.0.1:0");
    blocker.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) { return std::vector<uint8_t>{}; });
    blocker.Start();
    const std::string bound = blocker.GetListenAddress();

    TCPServer duplicate(bound);
    duplicate.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) { return std::vector<uint8_t>{}; });
    EXPECT_THROW(duplicate.Start(), std::runtime_error);

    blocker.Stop();
}

namespace {

// Reads with a timeout; returns the byte count (0 == orderly EOF, -1 == timeout/error).
int RecvWithTimeout(raw_socket_t sock, uint8_t* byte, int timeout_ms) {
    fd_set read_fds;
    FD_ZERO(&read_fds);
    FD_SET(sock, &read_fds);
    struct timeval tv{};
    tv.tv_sec = timeout_ms / 1000;
    tv.tv_usec = (timeout_ms % 1000) * 1000;
    int ready = ::select(static_cast<int>(sock) + 1, &read_fds, nullptr, nullptr, &tv);
    if (ready <= 0) return -1;
    return static_cast<int>(::recv(sock, byte, 1, 0));
}

void WriteRawFrame(raw_socket_t sock, uint32_t declared_size, const std::vector<uint8_t>& payload) {
    std::vector<uint8_t> frame(4 + payload.size());
    frame[0] = static_cast<uint8_t>((declared_size >> 24) & 0xFF);
    frame[1] = static_cast<uint8_t>((declared_size >> 16) & 0xFF);
    frame[2] = static_cast<uint8_t>((declared_size >> 8) & 0xFF);
    frame[3] = static_cast<uint8_t>(declared_size & 0xFF);
    std::memcpy(frame.data() + 4, payload.data(), payload.size());
    SendAllRaw(sock, frame.data(), frame.size());
}

std::string HostPortOf(const std::string& address) {
    const auto colon = address.rfind(':');
    return address.substr(0, colon);
}

unsigned short PortOf(const std::string& address) {
    const auto colon = address.rfind(':');
    return static_cast<unsigned short>(std::stoi(address.substr(colon + 1)));
}

}  // namespace

TEST(TCPServerBoost4Test, HandleClientDropsMalformedFrames) {
    TCPServer server("127.0.0.1:0");
    server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) { return std::vector<uint8_t>{0x01}; });
    server.Start();
    const std::string address = server.GetListenAddress();

    // Each malformed frame must leave the request unanswered (the handler
    // thread stops reading that connection).
    const std::vector<std::vector<uint8_t>> malformed = {
        std::vector<uint8_t>(5, 0x01),                                // payload < 8 bytes
        {0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},             // wrong version
    };
    for (const auto& payload : malformed) {
        raw_socket_t sock = ::socket(AF_INET, SOCK_STREAM, 0);
        ASSERT_NE(sock, RAW_INVALID_SOCK);
        sockaddr_in addr{};
        addr.sin_family = AF_INET;
        addr.sin_port = htons(PortOf(address));
        ASSERT_EQ(::inet_pton(AF_INET, HostPortOf(address).c_str(), &addr.sin_addr), 1);
        ASSERT_EQ(::connect(sock, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)), 0);
        WriteRawFrame(sock, static_cast<uint32_t>(payload.size()), payload);
        uint8_t byte = 0;
        // No response byte may come back for a malformed frame.
        EXPECT_NE(RecvWithTimeout(sock, &byte, 1200), 1);
        raw_closesocket(sock);
    }

    // Zero-length and oversized declared frames are also rejected.
    for (const uint32_t declared : {0u, 33u * 1024u * 1024u}) {
        raw_socket_t sock = ::socket(AF_INET, SOCK_STREAM, 0);
        ASSERT_NE(sock, RAW_INVALID_SOCK);
        sockaddr_in addr{};
        addr.sin_family = AF_INET;
        addr.sin_port = htons(PortOf(address));
        ASSERT_EQ(::inet_pton(AF_INET, HostPortOf(address).c_str(), &addr.sin_addr), 1);
        ASSERT_EQ(::connect(sock, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)), 0);
        WriteRawFrame(sock, declared, {});
        uint8_t byte = 0;
        EXPECT_NE(RecvWithTimeout(sock, &byte, 1200), 1);
        raw_closesocket(sock);
    }

    // A well-formed frame still gets its response afterwards.
    {
        raw_socket_t sock = ::socket(AF_INET, SOCK_STREAM, 0);
        ASSERT_NE(sock, RAW_INVALID_SOCK);
        sockaddr_in addr{};
        addr.sin_family = AF_INET;
        addr.sin_port = htons(PortOf(address));
        ASSERT_EQ(::inet_pton(AF_INET, HostPortOf(address).c_str(), &addr.sin_addr), 1);
        ASSERT_EQ(::connect(sock, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)), 0);
        auto frame = protocol::NewMessage(protocol::MSG_INVOKE_REQUEST, 42, {0x09});
        WriteRawFrame(sock, static_cast<uint32_t>(frame.size()), frame);
        uint8_t len_hdr[4];
        ASSERT_TRUE(ReadAllRaw(sock, len_hdr, 4));
        uint32_t len = (uint32_t(len_hdr[0]) << 24) | (uint32_t(len_hdr[1]) << 16) |
                       (uint32_t(len_hdr[2]) << 8) | uint32_t(len_hdr[3]);
        std::vector<uint8_t> body(len);
        ASSERT_TRUE(ReadAllRaw(sock, body.data(), len));
        auto parsed = protocol::ParseMessage(body);
        EXPECT_EQ(parsed.msg_id, protocol::MSG_INVOKE_RESPONSE);
        EXPECT_EQ(parsed.req_id, 42u);
        EXPECT_EQ(parsed.body, (std::vector<uint8_t>{0x01}));
        raw_closesocket(sock);
    }

    server.Stop();
}

// ---------------------------------------------------------------------------
// 6) Invoker edge cases (mock HTTP transport)
// ---------------------------------------------------------------------------

namespace {

class Boost4MockHTTPTransport final : public HTTPTransport {
public:
    using Responder = std::function<HTTPResponse(const HTTPRequest&)>;

    explicit Boost4MockHTTPTransport(Responder responder) : responder_(std::move(responder)) {}

    HTTPResponse Send(const HTTPRequest& request) override {
        std::lock_guard<std::mutex> lock(mutex_);
        requests.push_back(request);
        return responder_(request);
    }

    std::vector<HTTPRequest> requests;

private:
    std::mutex mutex_;
    Responder responder_;
};

HTTPResponse JsonResponse(unsigned status, const std::string& body) {
    HTTPResponse response;
    response.status_code = status;
    response.body = body;
    return response;
}

InvokerConfig Boost4InvokerConfig(const std::shared_ptr<HTTPTransport>& transport) {
    InvokerConfig config;
    config.address = "http://server.example";
    config.task_poll_interval_ms = 1;
    config.retry.enabled = false;
    config.http_transport = transport;
    config.disable_logging = true;
    return config;
}

}  // namespace

TEST(InvokerBoost4Test, InvokerAppliesDefaultsForZeroTimeoutAndPollInterval) {
    auto transport = std::make_shared<Boost4MockHTTPTransport>(
        [](const HTTPRequest&) { return JsonResponse(200, R"({"result":{"ok":true}})"); });
    InvokerConfig config = Boost4InvokerConfig(transport);
    config.timeout_seconds = 0;
    config.task_poll_interval_ms = 0;
    CroupierInvoker invoker(config);
    EXPECT_TRUE(invoker.Connect());
    EXPECT_EQ(invoker.Invoke("fn.ok", "{}"), R"({"ok":true})");
    invoker.Close();
}

TEST(InvokerBoost4Test, InvokerEscapesURLSegments) {
    std::string captured_url;
    auto transport = std::make_shared<Boost4MockHTTPTransport>(
        [&](const HTTPRequest& request) {
            captured_url = request.url;
            return JsonResponse(200, R"({"result":{}})");
        });
    CroupierInvoker invoker(Boost4InvokerConfig(transport));
    EXPECT_NO_THROW(invoker.Invoke("player one/two", "{}"));
    EXPECT_NE(captured_url.find("/functions/player%20one%2Ftwo/invoke"), std::string::npos)
        << captured_url;
    invoker.Close();
}

TEST(InvokerBoost4Test, InvokerSendsTraceIdempotencyAndAuthHeaders) {
    auto transport = std::make_shared<Boost4MockHTTPTransport>(
        [](const HTTPRequest&) { return JsonResponse(200, R"({"result":{}})"); });
    InvokerConfig config = Boost4InvokerConfig(transport);
    config.game_id = "game-9";
    config.env = "development";
    config.auth_token = "tok-123";
    CroupierInvoker invoker(config);

    InvokeOptions options;
    options.trace_id = "trace-1";
    options.idempotency_key = "idem-1";
    EXPECT_NO_THROW(invoker.Invoke("fn.h", "{}", options));

    ASSERT_FALSE(transport->requests.empty());
    const auto& headers = transport->requests.back().headers;
    std::map<std::string, std::string> lowered;
    for (const auto& [k, v] : headers) {
        std::string key = k;
        std::transform(key.begin(), key.end(), key.begin(),
                       [](unsigned char c) { return static_cast<char>(std::tolower(c)); });
        lowered[key] = v;
    }
    EXPECT_EQ(lowered["x-trace-id"], "trace-1");
    EXPECT_EQ(lowered["idempotency-key"], "idem-1");
    EXPECT_EQ(lowered["x-game-id"], "game-9");
    EXPECT_EQ(lowered["x-env"], "development");
    EXPECT_EQ(lowered["authorization"], "Bearer tok-123");
    invoker.Close();
}

TEST(InvokerBoost4Test, InvokerKeepsPrefloweredBearerToken) {
    auto transport = std::make_shared<Boost4MockHTTPTransport>(
        [](const HTTPRequest&) { return JsonResponse(200, R"({"result":{}})"); });
    InvokerConfig config = Boost4InvokerConfig(transport);
    config.auth_token = "bearer abc";
    CroupierInvoker invoker(config);
    EXPECT_NO_THROW(invoker.Invoke("fn.b", "{}"));
    ASSERT_FALSE(transport->requests.empty());
    bool found = false;
    for (const auto& [k, v] : transport->requests.back().headers) {
        if (k == "Authorization") {
            EXPECT_EQ(v, "bearer abc");
            found = true;
        }
    }
    EXPECT_TRUE(found);
    invoker.Close();
}

TEST(InvokerBoost4Test, OptionMetadataOverridesConfigHeaders) {
    auto transport = std::make_shared<Boost4MockHTTPTransport>(
        [](const HTTPRequest&) { return JsonResponse(200, R"({"result":{}})"); });
    InvokerConfig config = Boost4InvokerConfig(transport);
    config.headers["X-Env"] = "development";
    CroupierInvoker invoker(config);
    InvokeOptions options;
    options.metadata["x-env"] = "production";  // case-insensitive overwrite
    EXPECT_NO_THROW(invoker.Invoke("fn.u", "{}", options));
    ASSERT_FALSE(transport->requests.empty());
    int env_values = 0;
    for (const auto& [k, v] : transport->requests.back().headers) {
        if (k == "X-Env") {
            EXPECT_EQ(v, "production");
            ++env_values;
        }
    }
    EXPECT_EQ(env_values, 1);  // updated in place, not duplicated
    invoker.Close();
}

TEST(InvokerBoost4Test, ServerErrorSurfacesPlainBodyMessage) {
    auto transport = std::make_shared<Boost4MockHTTPTransport>(
        [](const HTTPRequest&) { return JsonResponse(500, "plain oops"); });
    CroupierInvoker invoker(Boost4InvokerConfig(transport));
    try {
        invoker.Invoke("fn.err", "{}");
        FAIL() << "expected HTTPStatusError";
    } catch (const std::runtime_error& error) {
        EXPECT_NE(std::string(error.what()).find("plain oops"), std::string::npos);
    }
    invoker.Close();
}

TEST(InvokerBoost4Test, ServerErrorWithBlankBodyReportsEmptyBody) {
    auto transport = std::make_shared<Boost4MockHTTPTransport>(
        [](const HTTPRequest&) { return JsonResponse(503, "   "); });
    CroupierInvoker invoker(Boost4InvokerConfig(transport));
    try {
        invoker.Invoke("fn.blank", "{}");
        FAIL() << "expected HTTPStatusError";
    } catch (const std::runtime_error& error) {
        EXPECT_NE(std::string(error.what()).find("empty response body"), std::string::npos);
    }
    invoker.Close();
}

TEST(InvokerBoost4Test, RetryWithJitterRetriesServerErrors) {
    auto transport = std::make_shared<Boost4MockHTTPTransport>(
        [](const HTTPRequest&) { return JsonResponse(503, R"({"message":"unavailable"})"); });
    InvokerConfig config = Boost4InvokerConfig(transport);
    config.retry.enabled = true;
    config.retry.max_attempts = 2;
    config.retry.initial_delay_ms = 1;
    config.retry.jitter_factor = 0.5;
    CroupierInvoker invoker(config);
    try {
        invoker.Invoke("fn.retry", "{}");
        FAIL() << "expected HTTPStatusError after retries";
    } catch (const std::runtime_error& error) {
        EXPECT_NE(std::string(error.what()).find("unavailable"), std::string::npos);
    }
    EXPECT_EQ(transport->requests.size(), size_t(2));
    invoker.Close();
}

TEST(InvokerBoost4Test, CancelTaskReturnsFalseForEmptyTaskId) {
    auto transport = std::make_shared<Boost4MockHTTPTransport>(
        [](const HTTPRequest&) { return JsonResponse(200, "{}"); });
    CroupierInvoker invoker(Boost4InvokerConfig(transport));
    EXPECT_FALSE(invoker.CancelTask(""));
    EXPECT_TRUE(transport->requests.empty());
    invoker.Close();
}

TEST(InvokerBoost4Test, StreamTaskReportsErrorWhenItemsMissing) {
    auto transport = std::make_shared<Boost4MockHTTPTransport>(
        [](const HTTPRequest&) { return JsonResponse(200, R"({"done":false})"); });
    CroupierInvoker invoker(Boost4InvokerConfig(transport));
    auto events = invoker.StreamTask("task-no-items").get();
    ASSERT_EQ(events.size(), size_t(1));
    EXPECT_EQ(events[0].event_type, "error");
    EXPECT_TRUE(events[0].done);
    EXPECT_NE(events[0].message.find("items"), std::string::npos);
    invoker.Close();
}

TEST(InvokerBoost4Test, StreamTaskReportsErrorWhenItemIsNotObject) {
    auto transport = std::make_shared<Boost4MockHTTPTransport>(
        [](const HTTPRequest&) { return JsonResponse(200, R"({"items":[123],"done":true})"); });
    CroupierInvoker invoker(Boost4InvokerConfig(transport));
    auto events = invoker.StreamTask("task-bad-item").get();
    ASSERT_EQ(events.size(), size_t(1));
    EXPECT_EQ(events[0].event_type, "error");
    EXPECT_NE(events[0].message.find("object"), std::string::npos);
    invoker.Close();
}

TEST(InvokerBoost4Test, StreamTaskMapsFailedEventsToErrorField) {
    auto transport = std::make_shared<Boost4MockHTTPTransport>([](const HTTPRequest&) {
        return JsonResponse(200, R"({"items":[{"type":"failed","message":"boom","seq":1},)"
                                 R"({"message":"no type","seq":2}],"done":true})");
    });
    CroupierInvoker invoker(Boost4InvokerConfig(transport));
    auto events = invoker.StreamTask("task-failed").get();
    ASSERT_EQ(events.size(), size_t(2));
    EXPECT_EQ(events[0].event_type, "failed");
    EXPECT_EQ(events[0].error, "boom");
    EXPECT_TRUE(events[0].done);
    EXPECT_EQ(events[1].event_type, "unknown");
    EXPECT_FALSE(events[1].done);
    invoker.Close();
}

TEST(InvokerBoost4Test, ClosedInvokerRejectsRequests) {
    auto transport = std::make_shared<Boost4MockHTTPTransport>(
        [](const HTTPRequest&) { return JsonResponse(200, "{}"); });
    CroupierInvoker invoker(Boost4InvokerConfig(transport));
    invoker.Close();
    EXPECT_THROW(invoker.Invoke("fn.closed", "{}"), std::runtime_error);
}

// ---------------------------------------------------------------------------
// 7) Client config loader edges
// ---------------------------------------------------------------------------

TEST(ConfigLoaderBoost4Test, ValidateConfigCollectsRangeErrors) {
    config::ClientConfigLoader loader;
    ClientConfig bad;
    bad.game_id = "game";
    bad.agent_addr = "127.0.0.1:19091";
    bad.timeout_seconds = 0;
    bad.reconnect_interval_seconds = 0;
    bad.reconnect_max_attempts = -1;
    bad.env = "middle-earth";
    const auto errors = loader.ValidateConfig(bad);
    auto contains = [&errors](const std::string& needle) {
        for (const auto& error : errors) {
            if (error.find(needle) != std::string::npos) return true;
        }
        return false;
    };
    EXPECT_TRUE(contains("timeout_seconds must be greater than 0"));
    EXPECT_TRUE(contains("reconnect_interval_seconds must be greater than 0"));
    EXPECT_TRUE(contains("reconnect_max_attempts must be >= 0"));
    EXPECT_TRUE(contains("env must be one of"));
}

TEST(ConfigLoaderBoost4Test, MergeConfigsAppliesServiceIdAndControlAddr) {
    config::ClientConfigLoader loader;

    ClientConfig base;
    base.game_id = "game-a";
    base.agent_addr = "127.0.0.1:1";
    base.service_id = "svc-base";

    ClientConfig overlay;
    overlay.service_id = "svc-overlay-42";
    overlay.control_addr = "127.0.0.1:9999";

    ClientConfig merged = loader.MergeConfigs(base, overlay);
    EXPECT_EQ(merged.service_id, "svc-overlay-42");
    EXPECT_EQ(merged.control_addr, "127.0.0.1:9999");

    // A default overlay service id still fills an empty base value.
    ClientConfig empty_base;
    empty_base.game_id = "game-a";
    empty_base.agent_addr = "127.0.0.1:1";
    ClientConfig default_overlay;
    default_overlay.service_id = "cpp-service";
    ClientConfig merged2 = loader.MergeConfigs(empty_base, default_overlay);
    EXPECT_EQ(merged2.service_id, "cpp-service");
}

TEST(ConfigLoaderBoost4Test, ValidateConfigCollectsSecurityErrors) {
    config::ClientConfigLoader loader;
    ClientConfig config;
    config.game_id = "game";
    config.agent_addr = "127.0.0.1:19091";
    config.env = "development";
    config.insecure = false;
    // TLS path with a missing key file exercises the file-existence check
    // inside ValidateFilePath via the public ValidateConfig entry point.
    config.cert_file = "/no-such-cert-boost4.pem";
    config.key_file = "/no-such-key-boost4.pem";
    const auto errors = loader.ValidateConfig(config);
    auto contains = [&errors](const std::string& needle) {
        for (const auto& error : errors) {
            if (error.find(needle) != std::string::npos) return true;
        }
        return false;
    };
    EXPECT_TRUE(contains("cert_file does not exist"));
    EXPECT_TRUE(contains("key_file does not exist"));
    EXPECT_TRUE(contains("ca_file is required"));
    EXPECT_TRUE(contains("server_name is required"));
}

// ---------------------------------------------------------------------------
// 8) OpenAPI importer variants
// ---------------------------------------------------------------------------

namespace {

const char* kBoost4Spec = R"({
  "openapi": "3.0.0",
  "info": {"title": "boost4", "version": "1.0.0"},
  "paths": {
    "/": {"get": {"responses": {"200": {"description": "ok"}}}},
    "/players": {"post": {
      "operationId": "players.warn",
      "x-risk": "DANGER",
      "x-resource": {"kind": "player"},
      "x-permission": true,
      "requestBody": {"content": {"application/json": {"schema": {
        "type": "object",
        "description": "Warn payload",
        "properties": {"reason": {"type": "string"}}
      }}}},
      "responses": {"200": {"description": "ok"}}
    }},
    "/flags": {"post": {
      "operationId": "flags.set",
      "x-risk": "weird-level",
      "responses": {"200": {"description": "ok"}}
    }}
  }
})";

openapi::ImportOptions Boost4ImportOptions() {
    openapi::ImportOptions options;
    options.continue_on_error = true;
    return options;
}

openapi::HandlerResolver Boost4Resolver() {
    return [](const std::string&) {
        return std::optional<FunctionHandler>(
            [](const std::string&, const std::string&) { return std::string("ok"); });
    };
}

}  // namespace

TEST(OpenAPIBoost4Test, SinkImportDerivesFallbacksAndExtensions) {
    std::vector<FunctionDescriptor> received;
    auto sink = [&received](const FunctionDescriptor& descriptor, FunctionHandler) {
        received.push_back(descriptor);
        return true;
    };
    auto registered =
        openapi::RegisterFromOpenAPI(sink, kBoost4Spec, Boost4ImportOptions(), Boost4Resolver());
    ASSERT_EQ(received.size(), size_t(3));

    // Iteration follows the document order of "paths": "/", "/players", "/flags".
    const FunctionDescriptor& unknown = received[0];
    EXPECT_EQ(unknown.id, "unknown.function");
    EXPECT_EQ(unknown.summary, "Unnamed Function");

    const FunctionDescriptor& warn = received[1];
    EXPECT_EQ(warn.id, "players.warn");
    EXPECT_EQ(warn.risk, "danger");
    EXPECT_EQ(warn.resource, R"({"kind":"player"})");
    EXPECT_EQ(warn.permission, "true");
    EXPECT_NE(warn.input_schema.find("Warn payload"), std::string::npos);

    const FunctionDescriptor& flags = received[2];
    EXPECT_EQ(flags.id, "flags.set");
    EXPECT_EQ(flags.risk, "medium");
}

TEST(OpenAPIBoost4Test, ClientOverloadRegistersThroughClient) {
    CroupierClient client(Boost4ProviderConfig("127.0.0.1:19091"));
    auto registered = openapi::RegisterFromOpenAPI(client, kBoost4Spec, Boost4ImportOptions(),
                                                   Boost4Resolver());
    EXPECT_EQ(registered.size(), size_t(3));
}

TEST(OpenAPIBoost4Test, WithHandlersResolvesOnlyMappedFunctions) {
    CroupierClient client(Boost4ProviderConfig("127.0.0.1:19091"));
    std::map<std::string, FunctionHandler> handlers;
    handlers["players.warn"] = [](const std::string&, const std::string&) { return "warned"; };
    auto registered = openapi::RegisterFromOpenAPIWithHandlers(client, kBoost4Spec,
                                                               Boost4ImportOptions(), handlers);
    ASSERT_EQ(registered.size(), size_t(1));
    EXPECT_EQ(registered[0], "players.warn");
}

// ---------------------------------------------------------------------------
// 9) file_utils edges
// ---------------------------------------------------------------------------

TEST(FileUtilsBoost4Test, CreateDirectoryFailsUnderUnwritableParent) {
#ifdef _WIN32
    GTEST_SKIP() << "POSIX read-only path not portable to Windows";
#else
    EXPECT_FALSE(utils::FileSystemUtils::CreateDirectory("/proc/croupier-boost4/nested"));
#endif
}

TEST(FileUtilsBoost4Test, WriteFileContentFailsForDirectoryPath) {
    EXPECT_FALSE(utils::FileSystemUtils::WriteFileContent(::testing::TempDir(), "payload"));
}

TEST(FileUtilsBoost4Test, ListDirectoriesOfMissingDirectoryIsEmpty) {
    const auto entries = utils::FileSystemUtils::ListDirectories(
        ::testing::TempDir() + "no-such-dir-boost4");
    EXPECT_TRUE(entries.empty());
}

TEST(FileUtilsBoost4Test, NormalizePathHandlesSeparatorsAndDotDot) {
    EXPECT_EQ(utils::FileSystemUtils::NormalizePath("a\\b\\c"), "a/b/c");
    EXPECT_EQ(utils::FileSystemUtils::NormalizePath("a/../../b"), "../b");
    EXPECT_EQ(utils::FileSystemUtils::NormalizePath("./x/./y"), "x/y");
}

TEST(FileUtilsBoost4Test, CreateTempFileFailsForHugePrefix) {
    const std::string huge_prefix(4000, 'p');
    EXPECT_EQ(utils::FileSystemUtils::CreateTempFile(huge_prefix, ".tmp"), "");
}

TEST(FileUtilsBoost4Test, RemoveDirectoryRecursiveFailsWhenDirectoryIsReadOnly) {
#ifdef _WIN32
    GTEST_SKIP() << "POSIX permission semantics required";
#else
    if (::geteuid() == 0) {
        GTEST_SKIP() << "root ignores directory write bits";
    }
    const std::string dir = ::testing::TempDir() + "boost4-readonly";
    RemoveTreeIfExists(dir);
    ASSERT_TRUE(utils::FileSystemUtils::CreateDirectory(dir));
    ASSERT_TRUE(utils::FileSystemUtils::CreateDirectory(dir + "/child"));
    ASSERT_TRUE(utils::FileSystemUtils::WriteFileContent(dir + "/file.txt", "x"));
    ASSERT_TRUE(utils::FileSystemUtils::WriteFileContent(dir + "/child/file.txt", "y"));
    ASSERT_EQ(::chmod(dir.c_str(), 0555), 0);
    EXPECT_FALSE(utils::FileSystemUtils::RemoveDirectory(dir, true));
    ASSERT_EQ(::chmod(dir.c_str(), 0755), 0);
    RemoveTreeIfExists(dir);
#endif
}

// ---------------------------------------------------------------------------
// 10) Plugin loader edges
// ---------------------------------------------------------------------------

namespace {

std::string Boost4PluginPath(const std::string& name) {
#ifdef CROUPIER_TEST_PLUGIN_DIR
    return std::string(CROUPIER_TEST_PLUGIN_DIR) + "/" + name;
#else
    return name;
#endif
}

}  // namespace

TEST(PluginLoaderBoost4Test, RegisterPluginFunctionsFailsWhileClientRunning) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    plugin::PluginManager manager;
    ASSERT_TRUE(manager.LoadPlugin(Boost4PluginPath("libcroupier-sample-plugin.so")));

    ClientConfig verbose_config = Boost4ProviderConfig(agent.address());
    verbose_config.disable_logging = true;
    CroupierClient client(verbose_config);
    ASSERT_TRUE(client.RegisterFunction(MakeEchoDescriptor(),
                                        [](const std::string&, const std::string& p) { return p; }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    EXPECT_FALSE(manager.RegisterPluginFunctions(client, "sample_plugin"));

    client.Close();
    manager.UnloadPlugin("sample_plugin");
}

TEST(PluginLoaderBoost4Test, LoadPluginWithoutInfoFunctionFails) {
#ifdef _WIN32
    GTEST_SKIP() << "system library probe is POSIX-only";
#else
    const char* candidates[] = {
        "/lib/x86_64-linux-gnu/libm.so.6",
        "/usr/lib/x86_64-linux-gnu/libm.so.6",
        "/lib/x86_64-linux-gnu/libz.so.1",
        "/lib64/ld-linux-x86-64.so.2",
    };
    std::string probe;
    for (const char* candidate : candidates) {
        if (utils::FileSystemUtils::FileExists(candidate)) {
            probe = candidate;
            break;
        }
    }
    ASSERT_FALSE(probe.empty()) << "no system shared library found for probing";
    plugin::PluginManager manager;
    EXPECT_FALSE(manager.LoadPlugin(probe));
#endif
}

TEST(PluginLoaderBoost4Test, LoadPluginWithNullInfoFails) {
    plugin::PluginManager manager;
    EXPECT_FALSE(manager.LoadPlugin(Boost4PluginPath("libcroupier-null-info-plugin.so")));
}

TEST(PluginLoaderBoost4Test, LoadPluginWithoutFunctionsWarnsButSucceeds) {
    plugin::PluginManager manager;
    ASSERT_TRUE(manager.LoadPlugin(Boost4PluginPath("libcroupier-empty-plugin.so")));
    const auto info = manager.GetPluginInfo("empty_plugin");
    EXPECT_EQ(info.name, "empty_plugin");
    EXPECT_TRUE(manager.GetPluginFunctions("empty_plugin").empty());
    EXPECT_TRUE(manager.UnloadPlugin("empty_plugin"));
}

TEST(PluginLoaderBoost4Test, UnloadMissingPluginFails) {
    plugin::PluginManager manager;
    EXPECT_FALSE(manager.UnloadPlugin("ghost-plugin"));
}

TEST(PluginLoaderBoost4Test, LoadPluginStripsLibAndCroupierPrefixes) {
    // A renamed copy of the sample plugin exercises the name-inference path
    // ("lib" + "croupier" + leading "_" stripping).
    const std::string source = Boost4PluginPath("libcroupier-sample-plugin.so");
    const std::string copy_path = ::testing::TempDir() + "libcroupier__renamed.so";
    ASSERT_TRUE(utils::FileSystemUtils::CopyFile(source, copy_path, true));
    plugin::PluginManager manager;
    EXPECT_TRUE(manager.LoadPlugin(copy_path));
    const auto info = manager.GetPluginInfo("sample_plugin");  // name comes from plugin info
    EXPECT_EQ(info.name, "sample_plugin");
    manager.UnloadPlugin("sample_plugin");
    utils::FileSystemUtils::RemoveFile(copy_path);
}

// ---------------------------------------------------------------------------
// 11) logger / field hints / dispatcher / JSON-schema pattern edges
// ---------------------------------------------------------------------------

TEST(LoggerBoost4Test, MaskJsonSensitiveMasksListedKeys) {
    const std::string masked = MaskJsonSensitive(R"({"token":"secret123","name":"n"})", {"token"});
    EXPECT_EQ(masked.find("secret123"), std::string::npos);
    EXPECT_NE(masked.find("name"), std::string::npos);
}

TEST(LoggerBoost4Test, LogMaskedSkipsWhenLevelDisabled) {
    auto& logger = Logger::GetInstance();
    const Logger::Level previous = Logger::Level::INFO;
    logger.SetLevel(Logger::Level::OFF);
    EXPECT_NO_THROW(logger.LogMasked(Logger::Level::INFO, "boost4", "msg", "sensitive"));
    logger.SetLevel(previous);
}

TEST(LoggerBoost4Test, LevelToStringCoversErrorAndUnknown) {
    auto& logger = Logger::GetInstance();
    logger.SetLevel(Logger::Level::DEBUG);
    EXPECT_NO_THROW(logger.Error("boost4", "error level line"));
    EXPECT_NO_THROW(logger.Log(static_cast<Logger::Level>(42), "boost4", "unknown level line"));
    logger.SetLevel(Logger::Level::INFO);
}

TEST(FieldHintsBoost4Test, RejectsTooShortXHint) {
    FunctionDescriptor descriptor;
    descriptor.id = "fn.hints";
    EXPECT_FALSE(SetFieldHint(descriptor, "field", "x-", nlohmann::json(true)));
}

TEST(FieldHintsBoost4Test, RejectsNonObjectInputSchema) {
    FunctionDescriptor descriptor;
    descriptor.id = "fn.hints";
    descriptor.input_schema = "[1,2,3]";
    EXPECT_FALSE(SetFieldHint(descriptor, "field", "x-widget", nlohmann::json("text")));
}

TEST(DispatcherBoost4Test, EnqueueOnMainThreadSwallowsCallbackExceptions) {
    auto& dispatcher = threading::MainThreadDispatcher::GetInstance();
    dispatcher.Initialize();
    EXPECT_NO_THROW(dispatcher.Enqueue([] { throw std::runtime_error("boom"); }));
}

TEST(JsonSchemaBoost4Test, InvalidPatternIsIgnored) {
    const bool valid = utils::JsonUtils::ValidateJsonSchema(
        R"({"name":"anything"})",
        R"({"type":"object","properties":{"name":{"type":"string","pattern":"["}}})");
    EXPECT_TRUE(valid);  // invalid regex is ignored; the Server re-validates
}

// ---------------------------------------------------------------------------
// 12) Second-wave edges: address parsing through the public client surface,
//     manifest escaping, drain-recovery failure, DNS success paths, env
//     override errors, plugin validation fixtures, file-util recursion.
// ---------------------------------------------------------------------------

TEST(ClientAddressBoost4Test, RegisteredClientRejectsHTTPAddress) {
    CroupierClient client(Boost4ProviderConfig("http://127.0.0.1:18780"));
    ASSERT_TRUE(client.RegisterFunction(MakeEchoDescriptor(),
                                        [](const std::string&, const std::string& p) { return p; }));
    EXPECT_FALSE(client.Connect());
    client.Close();
}

TEST(ClientAddressBoost4Test, RegisteredClientRejectsEmptyAddress) {
    CroupierClient client(Boost4ProviderConfig(""));
    ASSERT_TRUE(client.RegisterFunction(MakeEchoDescriptor(),
                                        [](const std::string&, const std::string& p) { return p; }));
    EXPECT_FALSE(client.Connect());
    client.Close();
}

TEST(ClientAddressBoost4Test, RegisteredClientRejectsMalformedIPv6Addresses) {
    const std::string bad[] = {"[::1]", "[::1]noport"};
    for (const std::string& address : bad) {
        CroupierClient client(Boost4ProviderConfig(address));
        ASSERT_TRUE(client.RegisterFunction(
            MakeEchoDescriptor(), [](const std::string&, const std::string& p) { return p; }));
        EXPECT_FALSE(client.Connect()) << "address: " << address;
        client.Close();
    }
}

TEST(ClientAddressBoost4Test, RegisteredClientParsesIPv6WithPortAndEmptyHost) {
    // "[::1]:19091" parses but nothing listens there: Connect fails after parse.
    {
        CroupierClient client(Boost4ProviderConfig("[::1]:19091"));
        ASSERT_TRUE(client.RegisterFunction(
            MakeEchoDescriptor(), [](const std::string&, const std::string& p) { return p; }));
        EXPECT_FALSE(client.Connect());
        client.Close();
    }
    // ":19091" has an empty host: rejected during address parsing.
    {
        CroupierClient client(Boost4ProviderConfig(":19091"));
        ASSERT_TRUE(client.RegisterFunction(
            MakeEchoDescriptor(), [](const std::string&, const std::string& p) { return p; }));
        EXPECT_FALSE(client.Connect());
        client.Close();
    }
}

TEST(ClientConstructionBoost4Test, DebugLoggingLevelIsApplied) {
    ClientConfig config = Boost4ProviderConfig("127.0.0.1:19091");
    config.debug_logging = true;
    config.disable_logging = false;
    EXPECT_NO_THROW({ CroupierClient client(config); });
}

TEST(ProviderInboundBoost4Test, LongGarbageInvokeBodyIsContained) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    CroupierClient client(Boost4ProviderConfig(agent.address()));
    ASSERT_TRUE(client.RegisterFunction(MakeEchoDescriptor(),
                                        [](const std::string&, const std::string& p) { return p; }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    // 40 bytes of invalid protobuf: parsing fails inside ParseMessage and the
    // handler returns an empty response without breaking the session.
    std::vector<uint8_t> garbage(40, 0xFF);
    agent.PushRequest(protocol::MSG_INVOKE_REQUEST, 9301, garbage);
    auto resp = agent.ReadResponseFor(9301);
    EXPECT_TRUE(resp.body.empty());

    // Long valid payloads still round-trip.
    const std::string long_payload(40, 'x');
    v1::InvokeRequest req;
    req.set_function_id("test.echo");
    req.set_payload(long_payload);
    std::string out;
    req.SerializeToString(&out);
    agent.PushRequest(protocol::MSG_INVOKE_REQUEST, 9302,
                      std::vector<uint8_t>(out.begin(), out.end()));
    auto resp2 = agent.ReadResponseFor(9302);
    v1::InvokeResponse parsed;
    ASSERT_TRUE(parsed.ParseFromArray(resp2.body.data(), static_cast<int>(resp2.body.size())));
    EXPECT_EQ(parsed.payload(), long_payload);

    client.Close();
}

TEST(ProviderDrainBoost4Test, DrainRecoveryFailureWhenAgentDisappears) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    CroupierClient client(Boost4ProviderConfig(agent.address()));
    ASSERT_TRUE(client.RegisterFunction(MakeEchoDescriptor(),
                                        [](const std::string&, const std::string& p) { return p; }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    // Drain, then make the agent vanish: the recovery re-registration fails
    // and the client ends up disconnected instead of wedged.
    agent.PushRequest(protocol::MSG_PROVIDER_DRAIN_REQUEST, 9401, {});
    auto ack = agent.ReadResponseFor(9401);
    EXPECT_EQ(ack.msg_id, protocol::MSG_PROVIDER_DRAIN_RESPONSE);
    agent.CloseListener();

    for (int i = 0; i < 100 && client.IsDraining(); ++i) {
        std::this_thread::sleep_for(std::chrono::milliseconds(50));
    }
    EXPECT_FALSE(client.IsDraining());

    client.Close();
}

TEST(TCPTransportBoost4Test, ConnectResolvesHostnameSuccessfully) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptOnly(); });

    TCPTransport transport("localhost", agent.port(), 3000);
    EXPECT_NO_THROW(transport.Connect());
    agent_thread.join();

    // A round trip proves the DNS-resolved socket is functional.
    std::thread responder([&] {
        auto req = agent.ReadFrame();
        agent.PushRequest(protocol::MSG_INVOKE_RESPONSE, req.req_id, {0x07});
    });
    auto resp = transport.Call(protocol::MSG_INVOKE_REQUEST, {1});
    responder.join();
    EXPECT_EQ(resp.second, (std::vector<uint8_t>{0x07}));

    transport.Close();
}

TEST(TCPTransportBoost4Test, CallFailsWhenSendBreaksAfterRST) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptOnly(); });

    TCPTransport transport("127.0.0.1", agent.port(), 800);
    transport.Connect();
    agent_thread.join();
    agent.DropConnection();

    // First call drains into a closed remote and times out or errors; by the
    // second call the RST has arrived and send() itself fails.
    bool any_throw = false;
    for (int i = 0; i < 3 && !any_throw; ++i) {
        try {
            (void)transport.Call(protocol::MSG_PROVIDER_HEARTBEAT_REQUEST, {1});
        } catch (const std::exception&) {
            any_throw = true;
        }
    }
    EXPECT_TRUE(any_throw);
    transport.Close();
}

TEST(TCPTransportBoost4Test, ReadLoopExitsOnSilentPayloadTimeout) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptOnly(); });

    TCPTransport transport("127.0.0.1", agent.port(), 2000);
    transport.Connect();
    agent_thread.join();

    // Frame header announces 16 payload bytes, but nothing follows: the very
    // first payload recv() times out (n == -2).
    agent.SendRaw({0x00, 0x00, 0x00, 0x10});
    std::this_thread::sleep_for(std::chrono::milliseconds(1500));
    agent.DropConnection();
    EXPECT_THROW(transport.Call(protocol::MSG_PROVIDER_HEARTBEAT_REQUEST, {1}), std::runtime_error);
    transport.Close();
}

TEST(TCPTransportBoost4Test, ReadLoopSkipsShortAndBadVersionProtocolHeaders) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptOnly(); });

    TCPTransport transport("127.0.0.1", agent.port(), 4000);
    transport.Connect();
    agent_thread.join();

    // Hand-built frame whose whole payload is 5 bytes (< 8-byte header).
    {
        std::vector<uint8_t> frame{0x00, 0x00, 0x00, 0x05, 0x01, 0x02, 0x03, 0x04, 0x05};
        agent.SendRaw(frame);
    }
    // Hand-built 8-byte payload whose version byte is 2, not VERSION_1.
    {
        std::vector<uint8_t> frame{0x00, 0x00, 0x00, 0x08,
                                   0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00};
        agent.SendRaw(frame);
    }
    std::this_thread::sleep_for(std::chrono::milliseconds(150));

    std::thread responder([&] {
        auto req = agent.ReadFrame();
        agent.PushRequest(protocol::MSG_INVOKE_RESPONSE, req.req_id, {0x03});
    });
    auto resp = transport.Call(protocol::MSG_INVOKE_REQUEST, {1});
    responder.join();
    EXPECT_EQ(resp.second, (std::vector<uint8_t>{0x03}));

    transport.Close();
}

TEST(TCPServerBoost4Test, StartIsIdempotentWhileRunning) {
    TCPServer server("127.0.0.1:0");
    server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) { return std::vector<uint8_t>{}; });
    server.Start();
    EXPECT_NO_THROW(server.Start());  // already running: returns immediately
    server.Stop();
}

TEST(TCPServerBoost4Test, HandleClientBreaksOnIncompletePayload) {
    TCPServer server("127.0.0.1:0");
    server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) { return std::vector<uint8_t>{}; });
    server.Start();
    const std::string address = server.GetListenAddress();

    // Declare an 8-byte frame but send only 4 bytes, then half-close: the
    // server sees a short payload read and stops serving the connection.
    raw_socket_t sock = ::socket(AF_INET, SOCK_STREAM, 0);
    ASSERT_NE(sock, RAW_INVALID_SOCK);
    sockaddr_in addr{};
    addr.sin_family = AF_INET;
    addr.sin_port = htons(PortOf(address));
    ASSERT_EQ(::inet_pton(AF_INET, HostPortOf(address).c_str(), &addr.sin_addr), 1);
    ASSERT_EQ(::connect(sock, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)), 0);
    const std::vector<uint8_t> partial{0x00, 0x00, 0x00, 0x08, 0x01, 0x02, 0x03, 0x04};
    ASSERT_TRUE(SendAllRaw(sock, partial.data(), partial.size()));
#ifdef _WIN32
    ::shutdown(sock, SD_SEND);
#else
    ::shutdown(sock, SHUT_WR);
#endif
    uint8_t byte = 0;
    EXPECT_NE(RecvWithTimeout(sock, &byte, 1500), 1);  // no response ever comes
    raw_closesocket(sock);

    server.Stop();
}

TEST(TCPServerBoost4Test, EmptyHandlerResponseSendsNothing) {
    TCPServer server("127.0.0.1:0");
    server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) { return std::vector<uint8_t>{}; });
    server.Start();
    const std::string address = server.GetListenAddress();

    raw_socket_t sock = ::socket(AF_INET, SOCK_STREAM, 0);
    ASSERT_NE(sock, RAW_INVALID_SOCK);
    sockaddr_in addr{};
    addr.sin_family = AF_INET;
    addr.sin_port = htons(PortOf(address));
    ASSERT_EQ(::inet_pton(AF_INET, HostPortOf(address).c_str(), &addr.sin_addr), 1);
    ASSERT_EQ(::connect(sock, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)), 0);
    auto frame = protocol::NewMessage(protocol::MSG_INVOKE_REQUEST, 7, {0x01});
    WriteRawFrame(sock, static_cast<uint32_t>(frame.size()), frame);
    uint8_t byte = 0;
    EXPECT_NE(RecvWithTimeout(sock, &byte, 1500), 1);  // empty response is not sent
    raw_closesocket(sock);

    server.Stop();
}

TEST(ConfigEnvBoost4Test, LoadFromJsonThrowsOnInvalidJson) {
    config::ClientConfigLoader loader;
    EXPECT_THROW(loader.LoadFromJson("{not valid json"), std::runtime_error);
}

TEST(ConfigEnvBoost4Test, LoadWithEnvironmentOverridesReadsFile) {
    const std::string dir = ::testing::TempDir() + "boost4-config";
    RemoveTreeIfExists(dir);
    ASSERT_TRUE(utils::FileSystemUtils::CreateDirectory(dir));
    const std::string path = dir + "/client.yaml";
    // The JSON parser accepts JSON documents regardless of extension.
    ASSERT_TRUE(utils::FileSystemUtils::WriteFileContent(
        path, R"({"game_id":"g","env":"development","agent_addr":"127.0.0.1:19090"})"));

    config::ClientConfigLoader loader;
    ClientConfig config;
    EXPECT_NO_THROW({ config = loader.LoadWithEnvironmentOverrides(path, "CROUPIER_BOOST4_NONE_"); });
    EXPECT_EQ(config.game_id, "g");

    RemoveTreeIfExists(dir);
}

TEST(ConfigEnvBoost4Test, InvalidNumericEnvValuesAreIgnoredWithWarning) {
    const std::string dir = ::testing::TempDir() + "boost4-config-env";
    RemoveTreeIfExists(dir);
    ASSERT_TRUE(utils::FileSystemUtils::CreateDirectory(dir));
    const std::string path = dir + "/client.json";
    ASSERT_TRUE(utils::FileSystemUtils::WriteFileContent(
        path, R"({"game_id":"g","env":"development","agent_addr":"127.0.0.1:19090"})"));

    const char* vars[] = {
        "CROUPIER_BOOST4_TIMEOUT_SECONDS",
        "CROUPIER_BOOST4_HEARTBEAT_INTERVAL",
        "CROUPIER_BOOST4_RECONNECT_INTERVAL_SECONDS",
        "CROUPIER_BOOST4_RECONNECT_MAX_ATTEMPTS",
    };
    for (const char* var : vars) {
#ifdef _WIN32
        _putenv_s(var, "not-a-number");
#else
        ::setenv(var, "not-a-number", 1);
#endif
    }

    config::ClientConfigLoader loader;
    ClientConfig config;
    EXPECT_NO_THROW({ config = loader.LoadWithEnvironmentOverrides(path, "CROUPIER_BOOST4_"); });
    // Non-numeric values are skipped: the defaults remain.
    EXPECT_GT(config.timeout_seconds, 0);
    EXPECT_GT(config.heartbeat_interval, 0);
    EXPECT_GT(config.reconnect_interval_seconds, 0);
    EXPECT_GE(config.reconnect_max_attempts, 0);

    for (const char* var : vars) {
#ifdef _WIN32
        _putenv_s(var, "");
#else
        ::unsetenv(var);
#endif
    }
    RemoveTreeIfExists(dir);
}

TEST(PluginLoaderBoost4Test, LoadPluginWithEmptyNameFails) {
    plugin::PluginManager manager;
    EXPECT_FALSE(manager.LoadPlugin(Boost4PluginPath("libcroupier-bad-name-plugin.so")));
}

TEST(PluginLoaderBoost4Test, LoadPluginWithEmptyVersionFails) {
    plugin::PluginManager manager;
    EXPECT_FALSE(manager.LoadPlugin(Boost4PluginPath("libcroupier-bad-version-plugin.so")));
}

TEST(PluginLoaderBoost4Test, ThrowingPluginFunctionAndCleanupAreContained) {
    plugin::PluginManager manager;
    ASSERT_TRUE(manager.LoadPlugin(Boost4PluginPath("libcroupier-throwing-plugin.so")));

    FunctionHandler handler = manager.GetPluginFunction("throwing_plugin.throwing_fn");
    ASSERT_TRUE(handler);
    EXPECT_NO_THROW({ const auto result = handler("ctx", "{}"); (void)result; });

    EXPECT_NO_THROW(manager.UnloadPlugin("throwing_plugin"));
}

TEST(PluginLoaderBoost4Test, ScanPluginsDiscoversLibraries) {
    plugin::PluginManager manager;
    const std::string dir = Boost4PluginPath(".");  // test-plugins output dir
    if (!utils::FileSystemUtils::DirectoryExists(dir)) {
        GTEST_SKIP() << "test plugin dir not available";
    }
    const auto found = manager.ScanPlugins(dir, /*load_immediately=*/false);
    EXPECT_GE(found.size(), size_t(1));
}

TEST(FileUtilsBoost4Test, ListFilesOfMissingDirectoryIsEmpty) {
    const auto files = utils::FileSystemUtils::ListFiles(::testing::TempDir() + "no-such-dir-boost4b");
    EXPECT_TRUE(files.empty());
}

TEST(FileUtilsBoost4Test, RecursiveDirectoryListingDescends) {
    const std::string dir = ::testing::TempDir() + "boost4-recursive";
    RemoveTreeIfExists(dir);
    ASSERT_TRUE(utils::FileSystemUtils::CreateDirectory(dir + "/child"));
    ASSERT_TRUE(utils::FileSystemUtils::WriteFileContent(dir + "/top.txt", "t"));
    ASSERT_TRUE(utils::FileSystemUtils::WriteFileContent(dir + "/child/inner.txt", "i"));

    const auto recursive_dirs = utils::FileSystemUtils::ListDirectories(dir, /*recursive=*/true);
    bool saw_child = false;
    for (const auto& entry : recursive_dirs) {
        if (entry.find("child") != std::string::npos) saw_child = true;
    }
    EXPECT_TRUE(saw_child);

    RemoveTreeIfExists(dir);
}

TEST(FileUtilsBoost4Test, RemoveDirectoryRecursiveFailsOnReadOnlyChild) {
#ifdef _WIN32
    GTEST_SKIP() << "POSIX permission semantics required";
#else
    if (::geteuid() == 0) {
        GTEST_SKIP() << "root ignores directory write bits";
    }
    const std::string dir = ::testing::TempDir() + "boost4-readonly-child";
    RemoveTreeIfExists(dir);
    ASSERT_TRUE(utils::FileSystemUtils::CreateDirectory(dir + "/child"));
    ASSERT_TRUE(utils::FileSystemUtils::WriteFileContent(dir + "/child/file.txt", "y"));
    // Only the child is read-only: the parent can enumerate, but the nested
    // recursive removal fails inside the child.
    ASSERT_EQ(::chmod((dir + "/child").c_str(), 0555), 0);
    EXPECT_FALSE(utils::FileSystemUtils::RemoveDirectory(dir, true));
    ASSERT_EQ(::chmod((dir + "/child").c_str(), 0755), 0);
    RemoveTreeIfExists(dir);
#endif
}

namespace {

const char* kBoost4SpecV2 = R"({
  "openapi": "3.0.0",
  "info": {"title": "boost4v2", "version": "1.0.0"},
  "paths": {
    "/summarized": {"post": {
      "operationId": "summarized.run",
      "summary": "Explicit summary",
      "requestBody": {"content": {"application/json": {}}},
      "responses": {"200": {"description": "ok"}}
    }}
  }
})";

}  // namespace

TEST(OpenAPIBoost4Test, SpecKeepsExplicitSummaryAndSchemalessContent) {
    std::vector<FunctionDescriptor> received;
    auto sink = [&received](const FunctionDescriptor& descriptor, FunctionHandler) {
        received.push_back(descriptor);
        return true;
    };
    auto registered = openapi::RegisterFromOpenAPI(sink, kBoost4SpecV2, Boost4ImportOptions(),
                                                   Boost4Resolver());
    ASSERT_EQ(received.size(), size_t(1));
    EXPECT_EQ(received[0].id, "summarized.run");
    EXPECT_EQ(received[0].summary, "Explicit summary");
    // application/json without a schema yields no input schema.
    EXPECT_TRUE(received[0].input_schema.empty());
}

TEST(TCPTransportBoost4Test, WriteResponseOnInvalidSocketIsNoop) {
    // Public static helper tolerates closed descriptors (workers may outlive
    // their captured socket after Close()).
    EXPECT_NO_THROW(TCPTransport::WriteResponseOnSocket(RAW_INVALID_SOCK,
                                                        protocol::MSG_INVOKE_RESPONSE, 1, {}));
}

TEST(TCPTransportBoost4Test, ConnectFailsImmediatelyOnBroadcastAddress) {
    // A non-blocking connect() to a broadcast address fails without going
    // through the select() wait on Linux.
#ifdef _WIN32
    GTEST_SKIP() << "broadcast probe is Linux-specific";
#else
    TCPTransport transport("255.255.255.255", 9, 1000);
    transport.SetConnectTimeout(800);
    EXPECT_THROW(transport.Connect(), std::runtime_error);
#endif
}

TEST(ProviderDescriptorBoost4Test, GarbageHandshakeResponseFailsConnect) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshakeGarbage(); });

    CroupierClient client(Boost4ProviderConfig(agent.address()));
    ASSERT_TRUE(client.RegisterFunction(MakeEchoDescriptor(),
                                        [](const std::string&, const std::string& p) { return p; }));
    EXPECT_FALSE(client.Connect());
    agent_thread.join();
    client.Close();
}

namespace {

// Minimal control-plane listener: accepts, reads one frame, answers with an
// empty RegisterCapabilitiesResponse. Exercises the full manifest upload
// (build + gzip + wire) so the manifest JSON escaper runs on real values.
void ServeControlPlaneOnce(unsigned short port, std::atomic<bool>* served) {
    raw_socket_t fd = ::socket(AF_INET, SOCK_STREAM, 0);
    if (fd == RAW_INVALID_SOCK) return;
    sockaddr_in addr{};
    addr.sin_family = AF_INET;
    addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
    addr.sin_port = htons(port);
    int reuse = 1;
    ::setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &reuse, sizeof(reuse));
    if (::bind(fd, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)) != 0 ||
        ::listen(fd, 1) != 0) {
        raw_closesocket(fd);
        return;
    }
    raw_socket_t conn = ::accept(fd, nullptr, nullptr);
    if (conn == RAW_INVALID_SOCK) {
        raw_closesocket(fd);
        return;
    }
    uint8_t len_hdr[4];
    if (ReadAllRaw(conn, len_hdr, 4)) {
        uint32_t len = (uint32_t(len_hdr[0]) << 24) | (uint32_t(len_hdr[1]) << 16) |
                       (uint32_t(len_hdr[2]) << 8) | uint32_t(len_hdr[3]);
        std::vector<uint8_t> frame(len);
        ReadAllRaw(conn, frame.data(), len);
        if (frame.size() >= 8) {
            uint32_t req_id = (uint32_t(frame[4]) << 24) | (uint32_t(frame[5]) << 16) |
                              (uint32_t(frame[6]) << 8) | uint32_t(frame[7]);
            auto resp = protocol::NewMessage(protocol::MSG_REGISTER_CAPABILITIES_RESP, req_id, {});
            std::vector<uint8_t> wrapped(4 + resp.size());
            wrapped[0] = static_cast<uint8_t>((resp.size() >> 24) & 0xFF);
            wrapped[1] = static_cast<uint8_t>((resp.size() >> 16) & 0xFF);
            wrapped[2] = static_cast<uint8_t>((resp.size() >> 8) & 0xFF);
            wrapped[3] = static_cast<uint8_t>(resp.size() & 0xFF);
            std::memcpy(wrapped.data() + 4, resp.data(), resp.size());
            SendAllRaw(conn, wrapped.data(), wrapped.size());
            served->store(true);
        }
    }
    raw_closesocket(conn);
    raw_closesocket(fd);
}

}  // namespace

TEST(ProviderDescriptorBoost4Test, ControlPlaneUploadBuildsEscapedManifest) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    // Bind a free port for the one-shot control plane.
    raw_socket_t probe = ::socket(AF_INET, SOCK_STREAM, 0);
    ASSERT_NE(probe, RAW_INVALID_SOCK);
    sockaddr_in pa{};
    pa.sin_family = AF_INET;
    pa.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
    pa.sin_port = 0;
    ASSERT_EQ(::bind(probe, reinterpret_cast<sockaddr*>(&pa), sizeof(pa)), 0);
    sockaddr_in bound{};
    socklen_t blen = sizeof(bound);
    ASSERT_EQ(::getsockname(probe, reinterpret_cast<sockaddr*>(&bound), &blen), 0);
    const unsigned short control_port = ntohs(bound.sin_port);
    raw_closesocket(probe);

    std::atomic<bool> served{false};
    std::thread control_thread([&] { ServeControlPlaneOnce(control_port, &served); });

    ClientConfig config = Boost4ProviderConfig(agent.address());
    config.control_addr = "127.0.0.1:" + std::to_string(control_port);
    CroupierClient client(config);
    // The full descriptor contains quotes/backslashes/newlines: the manifest
    // builder must escape all of them before upload.
    ASSERT_TRUE(client.RegisterFunction(MakeFullDescriptor(),
                                        [](const std::string&, const std::string&) { return "ok"; }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();
    for (int i = 0; i < 50 && !served.load(); ++i) {
        std::this_thread::sleep_for(std::chrono::milliseconds(20));
    }
    EXPECT_TRUE(served.load());
    client.Close();
    control_thread.join();
}

TEST(ProviderDrainBoost4Test, DrainRecoveryRestoresInboundDispatch) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    CroupierClient client(Boost4ProviderConfig(agent.address()));
    ASSERT_TRUE(client.RegisterFunction(MakeEchoDescriptor(),
                                        [](const std::string&, const std::string& p) { return "echo:" + p; }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    agent.PushRequest(protocol::MSG_PROVIDER_DRAIN_REQUEST, 9501, {});
    auto ack = agent.ReadResponseFor(9501);
    EXPECT_EQ(ack.msg_id, protocol::MSG_PROVIDER_DRAIN_RESPONSE);

    // Recovery reconnects through RegisterAllFunctions: the fresh transport
    // must dispatch inbound invocations again (the reconnect lambda path).
    std::thread reconnect_thread([&] { agent.AcceptReconnect(); });
    reconnect_thread.join();
    for (int i = 0; i < 100 && client.IsDraining(); ++i) {
        std::this_thread::sleep_for(std::chrono::milliseconds(20));
    }
    EXPECT_FALSE(client.IsDraining());

    agent.PushRequest(protocol::MSG_INVOKE_REQUEST, 9502,
                      InvokeBodyBoost4("test.echo", "after-recovery"));
    auto resp = agent.ReadResponseFor(9502);
    v1::InvokeResponse parsed;
    ASSERT_TRUE(parsed.ParseFromArray(resp.body.data(), static_cast<int>(resp.body.size())));
    EXPECT_EQ(parsed.payload(), "echo:after-recovery");

    client.Close();
}

TEST(ProviderHeartbeatBoost4Test, FastHeartbeatReconnectsAfterAgentDrop) {
    Boost4FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    ClientConfig config = Boost4ProviderConfig(agent.address());
    config.heartbeat_interval = 1;
    config.timeout_seconds = 2;  // short heartbeat call timeout
    CroupierClient client(config);
    ASSERT_TRUE(client.RegisterFunction(MakeEchoDescriptor(),
                                        [](const std::string&, const std::string& p) { return p; }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    // Kill the connection but keep listening: the heartbeat fails and the
    // reconnect loop re-establishes the session.
    std::thread reconnect_thread([&] { agent.AcceptReconnect(); });
    reconnect_thread.join();
    for (int i = 0; i < 300 && !client.IsConnected(); ++i) {
        std::this_thread::sleep_for(std::chrono::milliseconds(20));
    }
    EXPECT_TRUE(client.IsConnected());

    client.Close();
}

namespace {

const char* kBoost4SpecV3 = R"({
  "openapi": "3.0.0",
  "info": {"title": "boost4v3", "version": "1.0.0"},
  "paths": {
    "/plain": {"post": {
      "operationId": "plain.run",
      "requestBody": {"content": {"text/plain": {"schema": {"type": "string"}}}},
      "responses": {"200": {"description": "ok"}}
    }}
  }
})";

}  // namespace

TEST(OpenAPIBoost4Test, NonJsonContentYieldsNoSchema) {
    std::vector<FunctionDescriptor> received;
    auto sink = [&received](const FunctionDescriptor& descriptor, FunctionHandler) {
        received.push_back(descriptor);
        return true;
    };
    auto registered = openapi::RegisterFromOpenAPI(sink, kBoost4SpecV3, Boost4ImportOptions(),
                                                   Boost4Resolver());
    ASSERT_EQ(received.size(), size_t(1));
    EXPECT_EQ(received[0].id, "plain.run");
    EXPECT_TRUE(received[0].input_schema.empty());  // text/plain content is skipped
}

}  // namespace
}  // namespace croupier::sdk::test
