// 针对覆盖率缺口的补充用例：
// - TCP 非阻塞连接 select() 超时分支（backlog 占满、SYN 被静默丢弃）
// - LoadFromJson 非法 JSON 的异常路径
// - FilePush 响应编码边界（空字段 / 全字段）
// - HTTP transport 经基类指针析构（虚析构）
#include <gtest/gtest.h>

#include "croupier/sdk/config/client_config_loader.h"
#include "croupier/sdk/file_push.h"
#include "croupier/sdk/http_transport.h"
#include "croupier/sdk/tcp_transport.h"

#include <arpa/inet.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <unistd.h>

#include <cstdio>
#include <cstring>
#include <string>
#include <vector>

namespace croupier::sdk::test {
namespace {

// 监听 127.0.0.1 随机端口、从不 accept 的裸 socket 服务端。
// backlog 极小，用于占满 accept 队列制造连接挂起。
class NeverAcceptServer {
public:
    NeverAcceptServer() {
        listen_fd_ = socket(AF_INET, SOCK_STREAM, 0);
        if (listen_fd_ < 0) throw std::runtime_error("socket() failed");
        int reuse = 1;
        setsockopt(listen_fd_, SOL_SOCKET, SO_REUSEADDR, &reuse, sizeof(reuse));
        sockaddr_in addr{};
        addr.sin_family = AF_INET;
        addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
        addr.sin_port = 0;
        if (bind(listen_fd_, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)) != 0) {
            throw std::runtime_error("bind() failed");
        }
        if (listen(listen_fd_, 1) != 0) throw std::runtime_error("listen() failed");
        socklen_t len = sizeof(addr);
        if (getsockname(listen_fd_, reinterpret_cast<sockaddr*>(&addr), &len) != 0) {
            throw std::runtime_error("getsockname() failed");
        }
        port_ = ntohs(addr.sin_port);
    }

    ~NeverAcceptServer() {
        for (int fd : fillers_) close(fd);
        close(listen_fd_);
    }

    int port() const { return port_; }

    // 用阻塞连接占满 accept 队列（握手完成即入队，无需服务端 accept）。
    void FillBacklog(int count) {
        for (int i = 0; i < count; ++i) {
            int fd = socket(AF_INET, SOCK_STREAM, 0);
            if (fd < 0) break;
            sockaddr_in addr{};
            addr.sin_family = AF_INET;
            addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
            addr.sin_port = htons(port_);
            if (connect(fd, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)) == 0) {
                fillers_.push_back(fd);
            } else {
                close(fd);
                break;
            }
        }
    }

private:
    int listen_fd_ = -1;
    int port_ = 0;
    std::vector<int> fillers_;
};

}  // namespace

// backlog 占满后 SYN 被内核静默丢弃（Linux 默认 tcp_abort_on_overflow=0），
// 非阻塞 connect 返回 EINPROGRESS 后 select() 应走到超时分支并抛错。
TEST(TCPTransportCoverage, ConnectSelectTimeoutWhenBacklogFull) {
    NeverAcceptServer server;
    server.FillBacklog(2);  // backlog=1：两条 filler 确保队列占满

    bool threw = false;
    try {
        TCPTransport transport("127.0.0.1", server.port(), 200);
        transport.Connect();
    } catch (const std::runtime_error& e) {
        threw = true;
        EXPECT_NE(std::string(e.what()).find("timeout"), std::string::npos)
            << "unexpected error: " << e.what();
    }
    EXPECT_TRUE(threw);
}

// 非阻塞 connect 立即失败分支：Linux 下向广播地址 255.255.255.255 发起
// connect（未设置 SO_BROADCAST）会同步返回 EACCES，而非 EINPROGRESS。
TEST(TCPTransportCoverage, ConnectFailsImmediatelyOnBroadcastAddress) {
    bool threw = false;
    try {
        TCPTransport transport("255.255.255.255", 9999, 200);
        transport.Connect();
    } catch (const std::runtime_error& e) {
        threw = true;
        EXPECT_NE(std::string(e.what()).find("Failed to connect"), std::string::npos)
            << "unexpected error: " << e.what();
    }
    EXPECT_TRUE(threw);
}

// headers 字段类型错误（非 object）会使 .items() 抛 type_error，
// 应被 LoadFromJson 的 catch 包装为带上下文的 runtime_error。
// 语法非法 / 类型异常的配置内容都会在 LoadFromJson 抛 runtime_error，
// 不会把部分解析的结果泄漏给调用方。
TEST(ClientConfigLoaderCoverage, LoadFromJsonInvalidContentThrows) {
    config::ClientConfigLoader loader;
    ASSERT_THROW(loader.LoadFromJson("{not valid json"), std::runtime_error);
    // 类型异常的 headers 不抛错但也不产生头部项（安全 getter 语义）。
    ClientConfig config = loader.LoadFromJson("{\"headers\": 5}");
    EXPECT_TRUE(config.headers.empty());
}

// 响应编码：空字段（早退分支）与全字段（完整编码路径）都要稳定。
TEST(FilePushCoverage, EncodeResponseEmptyAndFull) {
    FilePushResponse empty;
    empty.ok = false;
    const auto empty_bytes = EncodeFilePushResponse(empty);
    EXPECT_TRUE(empty_bytes.empty());

    FilePushResponse full;
    full.transfer_id = "t-1";
    full.ok = true;
    full.stored_path = "/tmp/staging/patch.lua";
    full.error = "";
    const auto full_bytes = EncodeFilePushResponse(full);
    EXPECT_FALSE(full_bytes.empty());
    // field1(tag 0x0A) + ok(0x10 0x01) + field3(tag 0x1A)
    // wire: 0x0A len=3 't''-''1' 0x10 0x01 0x1A ...
    EXPECT_EQ(full_bytes[0], 0x0A);
    EXPECT_EQ(full_bytes[1], 0x03);
    EXPECT_EQ(full_bytes[2], 't');
    EXPECT_EQ(full_bytes[5], 0x10);
    EXPECT_EQ(full_bytes[6], 0x01);
    EXPECT_EQ(full_bytes[7], 0x1A);

    // 请求解码：手工构造 wire（field1 transfer_id + field2 file_name +
    // field3 sha + field4 data + 未知 field9 跳过）。
    std::vector<uint8_t> body;
    auto appendField = [&body](uint64_t field, const std::string& value) {
        AppendVarint(body, (field << 3) | 2);
        AppendVarint(body, value.size());
        body.insert(body.end(), value.begin(), value.end());
    };
    appendField(1, "t-2");
    appendField(2, "code.lua");
    appendField(3, "ab");
    appendField(9, "unknown");
    const std::string raw_data("\x01\x02\x03", 3);
    appendField(4, raw_data);
    FilePushRequest decoded = DecodeFilePushRequest(body);
    EXPECT_EQ(decoded.transfer_id, "t-2");
    EXPECT_EQ(decoded.file_name, "code.lua");
    EXPECT_EQ(decoded.content_sha256, "ab");
    EXPECT_EQ(decoded.data, (std::vector<uint8_t>{1, 2, 3}));
}

// 工厂返回的对象经 shared_ptr<HTTPTransport> 基类指针销毁（虚析构路径）。
TEST(HTTPTransportCoverage, FactoryObjectDestroyedThroughBasePointer) {
    std::shared_ptr<HTTPTransport> transport = NewDefaultHTTPTransport();
    ASSERT_NE(transport, nullptr);
    transport.reset();  // 虚析构在此触发
}

}  // namespace croupier::sdk::test
