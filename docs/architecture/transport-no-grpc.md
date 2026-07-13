---
title: 传输层决策 — 不使用 gRPC
---

# 传输层决策:不使用 gRPC

## 状态

Accepted(已落地,不可逆)。**不要再推荐 gRPC。**

## 决策

Croupier 内部 RPC(Server ↔ Agent ↔ SDK Provider)**不使用 gRPC**,采用自研 TCP transport + protobuf 强契约。

## 背景:从 gRPC 的坑里重构出来

项目早期曾用 gRPC 作内部传输,后经历 `gRPC → NNG → TCP-only` 的重构,最终移除 gRPC。git 历史:

- `7b3b955d2` chore(deps): bump google.golang.org/grpc
- `07d0f9faa` refactor(proto): remove gRPC service definitions from example proto files
- `573b94038` refactor: remove NNG dependency, migrate to TCP-only transport
- `4d0f1ffe4` fix: complete TCP-only migration (P0/P1/P2 issues)
- `775398748` Remove croupier-proto references

## 为什么不推荐 gRPC

1. **体积过重**:gRPC debug 版本约 **1.7GB**,对游戏后端是不可接受的负担。
2. **依赖难治理**:gRPC 依赖链复杂,曾花**一周**仍未搞定依赖冲突。
3. **游戏后端不需要**:gRPC 的核心优势(跨语言 IDL、HTTP/2 多路复用、丰富生态)对单公司多游戏后端非必需;自研 TCP+proto 已满足强契约,且可控、可调试。
4. **历史教训**:从 gRPC 重构出来的代价很大,不应重蹈覆辙。

## 替代方案(已落地)

- 自研 TCP transport:length-prefix framing(`internal/transport/tcp`),4 字节 size + payload,单帧上限 32MB。
- protobuf 消息定义在 `proto/`,生成代码在 `pkg/pb`。
- 强契约由 proto IDL 保证;传输层轻量、可控、易调试。
- 如需 gRPC 才有的能力(例如统一 error 协议),在自研 RPC 层补齐(定义 `Status`/`RpcError` proto 消息),**而非引入 gRPC**。

## 约束

- **不要再推荐 gRPC**。新增 RPC 需求一律走自研 TCP+proto。
- `go.mod` 中的 `google.golang.org/grpc` 若为某库间接引入,应标注并定期评估能否移除;**不得新增直接 gRPC 用法**。
- 检测:`rg "google.golang.org/grpc" internal/` 应零命中业务传输代码(只允许 `go.mod` 间接依赖或 `docs/archive/` 归档代码)。
- 评估任何新传输方案时,**体积与依赖复杂度是硬约束**,优先级高于"生态丰富度"。
