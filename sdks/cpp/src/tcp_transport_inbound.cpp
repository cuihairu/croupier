/**
 * @file tcp_transport_inbound.cpp
 * @brief TCPTransport inbound dispatch for Croupier C++ SDK.
 */


#include "croupier/sdk/tcp_transport.h"
#include <algorithm>
#include <stdexcept>
#include <cstring>
#include <cstddef>
#include <iostream>

#ifdef _WIN32
#include <winsock2.h>
#include <ws2tcpip.h>
#include <BaseTsd.h>
#ifdef _MSC_VER
#pragma comment(lib, "ws2_32.lib")
#endif
// Windows doesn't have ssize_t, use intptr_t instead
typedef intptr_t ssize_t;
#else
#include <errno.h>
#include <arpa/inet.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <unistd.h>
#endif

namespace croupier {
namespace sdk {

// ---- Inbound dispatch ----

namespace {
std::atomic<int> g_inbound_worker_override{0};
}

void TCPTransport::SetInboundWorkerCount(int workers) {
    g_inbound_worker_override.store(workers);
}

int TCPTransport::InboundWorkerCount() {
    const int overrideWorkers = g_inbound_worker_override.load();
    if (overrideWorkers > 0) {
        return overrideWorkers;
    }
    unsigned int n = std::thread::hardware_concurrency();
    return n == 0 ? 2 : static_cast<int>(std::max(2u, n));
}

void TCPTransport::SetInboundHandler(InboundHandler handler) {
    inbound_handler_ = std::move(handler);
    // 已启动的池同步更新 handler（重连复用同一 transport 的场景）。
    if (inbound_pool_) {
        std::lock_guard<std::mutex> lock(inbound_pool_->mu);
        inbound_pool_->handler = inbound_handler_;
    }
    if (control_pool_) {
        std::lock_guard<std::mutex> lock(control_pool_->mu);
        control_pool_->handler = inbound_handler_;
    }
}

void TCPTransport::StartPoolWorkers(const std::shared_ptr<InboundPool>& pool, int workers) {
    // worker 捕获 pool 的 shared_ptr（不捕获 this）：Close/析构置 stopping
    // 后 worker 自行退出，pool 状态由最后一个引用负责释放——修复
    // 「worker for(;;) 永不退出、析构销毁被等待中的 mutex/cv」挂起。
    for (int i = 0; i < workers; ++i) {
        auto owned = pool;
        pool->threads.emplace_back([owned] {
            for (;;) {
                std::tuple<uint32_t, uint32_t, std::vector<uint8_t>> task;
                {
                    std::unique_lock<std::mutex> lock(owned->mu);
                    owned->cv.wait(lock, [&owned] { return !owned->queue.empty() || owned->stopping; });
                    if (owned->queue.empty()) {
                        return;  // stopping 且无积压：退出
                    }
                    task = std::move(owned->queue.front());
                    owned->queue.pop();
                }
                auto [mid, rid, tbody] = std::move(task);
                std::vector<uint8_t> resp;
                try {
                    if (owned->handler) {
                        resp = owned->handler(mid, rid, tbody);
                    }
                } catch (const std::exception& e) {
                    std::cerr << "[ERROR] inbound handler: " << e.what() << '\n';
                    resp.clear();
                }
                WriteResponseOnSocket(owned->sock, protocol::GetResponseMsgID(mid), rid, resp);
                owned->queued.fetch_sub(1);
            }
        });
    }
}

void TCPTransport::EnqueueOnPool(const std::shared_ptr<InboundPool>& pool, uint32_t msg_id,
                                 uint32_t req_id, std::vector<uint8_t>&& body) {
    pool->queued.fetch_add(1);
    {
        std::lock_guard<std::mutex> lock(pool->mu);
        pool->queue.emplace(msg_id, req_id, std::move(body));
    }
    pool->cv.notify_one();
}

void TCPTransport::DispatchInbound(uint32_t msg_id, uint32_t req_id, std::vector<uint8_t> body) {
    if (!inbound_handler_) {
        return;
    }
    // 双车道（CR-FIX-08）：控制消息（心跳/注册/drain）走独立单 worker
    // 车道，永不 fail-fast——业务队列打满（快速失败）不得饿死控制帧，
    // 否则 Agent 误判会话死亡，过载升级为连接雪崩。与 Go/JS/Java/
    // Python 同规则（protocol::IsControlRequest 同一清单）。
    if (protocol::IsControlRequest(msg_id)) {
        if (!control_pool_) {
            control_pool_ = std::make_shared<InboundPool>();
            control_pool_->handler = inbound_handler_;
            control_pool_->sock = socket_;
            StartPoolWorkers(control_pool_, 1);
        }
        EnqueueOnPool(control_pool_, msg_id, req_id, std::move(body));
        return;
    }
    // 业务车道：惰性启动固定 worker 池（默认 = 硬件并发数）。
    if (!inbound_pool_) {
        inbound_pool_ = std::make_shared<InboundPool>();
        inbound_pool_->handler = inbound_handler_;
        inbound_pool_->sock = socket_;
        StartPoolWorkers(inbound_pool_, InboundWorkerCount());
    }
    const int workers = InboundWorkerCount();
    if (inbound_pool_->queued.load() >= workers * 4) {
        // 队列满：立即回空响应，Agent 侧 failover 接管。
        std::cerr << "[WARN] inbound queue full, fast-failing req_id=" << req_id << '\n';
        WriteResponseOnSocket(socket_, protocol::GetResponseMsgID(msg_id), req_id, {});
        return;
    }
    EnqueueOnPool(inbound_pool_, msg_id, req_id, std::move(body));
}

// WriteResponseOnSocket 向指定 fd 尽力写一帧（fd 已关闭时仅返回 EBADF，
// 不影响调用方）。静态函数：worker 持 pool->sock 值，析构后仍安全。
void TCPTransport::WriteResponseOnSocket(socket_t sock, uint32_t resp_msg_id, uint32_t req_id,
                                         const std::vector<uint8_t>& body) {
    if (sock == INVALID_SOCKET_VALUE) {
        return;
    }
    try {
        auto frame = protocol::NewMessage(resp_msg_id, req_id, body);
        std::vector<uint8_t> wrapped(4 + frame.size());
        wrapped[0] = static_cast<uint8_t>((frame.size() >> 24) & 0xFF);
        wrapped[1] = static_cast<uint8_t>((frame.size() >> 16) & 0xFF);
        wrapped[2] = static_cast<uint8_t>((frame.size() >> 8) & 0xFF);
        wrapped[3] = static_cast<uint8_t>(frame.size() & 0xFF);
        std::memcpy(wrapped.data() + 4, frame.data(), frame.size());
        // C-014：send 返回值/异常不再整体吞掉——失败至少记录日志（fd 已
        // 关闭仍返回 EBADF 且不影响调用方，但错误不再不可见；不再裸
        // (void)send）。
        int sent = send(sock, reinterpret_cast<const char*>(wrapped.data()),
                        static_cast<int>(wrapped.size()), 0);
        if (sent < 0) {
#ifdef _WIN32
            std::cerr << "[WARN] TCPTransport: write response frame failed req_id=" << req_id
                      << " err=" << WSAGetLastError() << '\n';
#else
            std::cerr << "[WARN] TCPTransport: write response frame failed req_id=" << req_id
                      << " err=" << errno << '\n';
#endif
        }
    } catch (const std::exception& e) {
        std::cerr << "[WARN] TCPTransport: write response frame exception req_id=" << req_id
                  << " what=" << e.what() << '\n';
    } catch (...) {
        std::cerr << "[WARN] TCPTransport: write response frame unknown exception req_id=" << req_id << '\n';
    }
}

} // namespace sdk
} // namespace croupier
