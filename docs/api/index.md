---
title: API 概览
icon: code
order: 1
---

# API 概览

Croupier 提供以下 API 接口：

| 类型 | 说明 | 协议 |
| --- | --- | --- |
| REST API | 面向 Dashboard 与外部管理调用 | HTTP / HTTPS |
| Session Wire API | SDK 与 Agent、Agent 与 Server 之间的内部协议 | TCP + TLS |

## REST API

REST API 用于：

- Dashboard 管理界面
- 外部系统集成
- 查询与配置操作

**基础路径：** `/api/v1/`

**认证方式：** JWT Bearer Token

## 主要接口分类

### 函数管理
- 函数列表、详情、创建、删除
- 函数调用与执行状态
- 函数权限配置
- 函数政策管理

### 游戏管理
- 游戏配置
- 玩家管理
- 消息推送

### 系统管理
- Agent 注册与状态
- 用户认证与授权
- 审批流程
- 审计日志

## 相关文档

- [函数 API](./function.md)
- [审批 API](./approval.md)
- [认证 API](./auth.md)
