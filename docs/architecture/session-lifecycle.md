---
title: Session 生命周期
icon: timeline
order: 5
category:
  - 系统架构
tag:
  - 架构
  - session
  - lifecycle
---

# Session 生命周期

本文档定义 Croupier `shared session runtime` 的最小生命周期模型，重点统一：

- session 状态
- 状态切换条件
- `drain` 的状态机语义
- `SDK <-> Agent` 与 `Agent <-> Server` 的共同行为约束

本文档描述的是运行时模型，不是具体某个语言 SDK 的 API 细节。

## 目标

这份状态机文档要解决三个问题：

1. 什么叫“session 已建立”
2. 什么叫“session 正在 drain”
3. 连接断开、重连、宽限关闭时，双方分别该做什么

## 最小状态集合

v1 建议统一使用下面几个核心状态：

- `connecting`
- `handshaking`
- `active`
- `draining`
- `closed`
- `reconnecting`

说明：

- `connecting`
  - 底层连接正在建立
- `handshaking`
  - 连接已建立，但首帧握手和会话注册尚未完成
- `active`
  - session 可正常收发业务请求
- `draining`
  - session 不再接收新业务请求，但允许排空在途请求
- `closed`
  - 当前 session 已终止，不再可用
- `reconnecting`
  - 旧 session 已终止，运行时正在等待或尝试建立新连接

## 状态机总览

```text
connecting
    |
    v
handshaking
    |
    v
 active  ---------> draining ---------> closed
    |                    ^                 |
    |                    |                 |
    +--------------------+                 |
             connection lost / fatal       |
                                            v
                                      reconnecting
                                            |
                                            v
                                       connecting
```

补充说明：

- `active -> draining` 是优雅摘流路径
- `active -> closed` 是直接关闭路径
- `draining -> closed` 是排空完成或宽限期结束
- `closed -> reconnecting` 只适用于启用了自动重连的一方

## 各状态允许的行为

### connecting

允许：

- 建立底层 `tcp` 连接
- 初始化读写循环
- 启动建连超时

不允许：

- 发送业务请求
- 接收业务分发

### handshaking

允许：

- 发送首帧握手消息
- 协商 `session_id`、协议版本与能力
- 验证首帧 `MsgID` 与 protobuf body

不允许：

- 把连接当作可用业务 session
- 分配 `Invoke / Job / Ops`

进入 `active` 的前提是：

- 握手成功
- 获得有效 `session_id`
- 本端运行时完成 in-flight 表初始化

### active

允许：

- 双向发送 request/response
- 多个并发 in-flight 请求复用
- heartbeat
- 背压与过载反馈
- 接收 `drain` 信号

这是唯一一个允许接收新业务请求分配的状态。

### draining

这是最容易实现歧义的状态，定义如下：

- session 仍然存活
- heartbeat 仍然继续
- 控制消息仍然可以收发
- 已在途请求允许继续执行
- 不再向该 session 分配新的业务请求

允许：

- 完成已在途请求
- 返回最后一批 response / event
- 继续处理 session 级控制消息
- 在需要时返回 `draining` / `retry_after_ms`

不允许：

- 接收新的业务分发
- 把该 session 重新视为普通可用节点

### closed

含义：

- 当前 session 已完全失效
- 旧 `session_id` 不可继续使用
- 该连接上的未完成请求应被判定为失败、取消或失效

### reconnecting

允许：

- 按退避策略等待
- 重新发起连接
- 重新进入 `connecting`

不允许：

- 复用旧 `session_id`
- 假定旧 in-flight 会自动恢复

## drain 状态机

`drain` 不是单独的一条控制命令而已，它对应的是 `active -> draining -> closed` 这条运行时路径。

### 触发条件

典型触发条件：

- 节点准备发布或退出
- 节点准备切换到新 session
- 节点过载，决定不再接新请求
- 运维侧主动执行摘流

### 进入 draining 时必须发生的事

当 session 从 `active` 进入 `draining`，运行时至少需要完成：

1. 标记 `draining = true`
2. 停止向该 session 分配新的业务请求
3. 保留 heartbeat
4. 保留 in-flight 跟踪
5. 启动可选的 `drain grace timeout`

### draining 期间的行为

在 `draining` 期间：

- 已发出的请求可以继续完成
- 新来的业务请求应被拒绝、重路由或明确告知稍后重试
- 不应再把该 session 参与正常调度权重

### 退出 draining 的条件

退出 `draining` 只允许两种结果：

1. 排空完成，进入 `closed`
2. 宽限时间结束，强制关闭后进入 `closed`

v1 不建议支持：

- `draining -> active`

原因是这会引入更多实现复杂度和调度歧义。  
如果未来确实需要“取消摘流”，应通过新 session 重建，而不是把旧 session 再改回 `active`。

## 在两条链路中的映射

### SDK <-> Agent

当 provider session 进入 `draining`：

- `Agent` 不再向该 provider 分配新的 `Invoke / Job`
- provider 继续完成已接收的请求
- `ProviderDrainResponse` 只表示“已接受 drain 状态”
- 排空完成后，连接可被优雅关闭

### Agent <-> Server

当 agent session 进入 `draining`：

- `Dispatcher` 不再向该 session 路由新的 `Invoke / Job / Ops`
- `Agent` 继续完成已接收的工作
- registry / 调度层应将该 session 视为“可排空，不可接新单”
- 排空完成后再移除该 session

## 失败与异常路径

不是所有关闭都走 `drain`。

以下情况通常直接进入 `closed`：

- 首帧非法
- 协议版本不兼容
- 认证失败
- 底层连接断开
- 致命编解码错误

这些路径不要求先进入 `draining`。

## in-flight 请求处理规则

当 session 关闭时，必须明确旧 in-flight 的处理原则：

- response 已返回的请求正常完成
- 未返回的请求应被标记为失败或取消
- 不允许在新 session 上隐式继承旧 in-flight

如果未来要做幂等恢复，应通过明确的请求级设计实现，而不是把 session 重连当成隐式恢复。

## 观测与指标建议

建议至少暴露以下运行时指标：

- 当前状态
- `session_id`
- `draining`
- `inflight_count`
- `queued_request_count`
- `last_heartbeat_at`
- `drain_started_at`
- `drain_deadline_at`
- `reconnect_attempt`

这样排障时才能判断：

- 它是真的死了
- 还是正在优雅摘流
- 还是正在退避重连

## 测试建议

最小测试集至少包括：

1. 建连成功后进入 `active`
2. 非法首帧直接进入 `closed`
3. `active` 下可并发收发多个 request/response
4. 进入 `draining` 后不再接收新业务请求
5. `draining` 期间已在途请求仍可完成
6. 宽限期后强制关闭剩余请求
7. 断线后进入 `reconnecting`
8. 重连后拿到新 `session_id`

## 最终约束

整个 v1 生命周期模型可以压缩成一句话：

- session 建好后进入 `active`
- 要优雅退出就进入 `draining`
- 一旦连接断开或宽限期结束就进入 `closed`
- 若启用自动重连，再从 `reconnecting` 建立新 session

这套模型必须同时适用于：

- `sdk-agent subprotocol`
- `agent-server subprotocol`

差异只在握手消息和默认 TLS 策略，不在生命周期语义。
