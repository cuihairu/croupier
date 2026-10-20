---
layout: home

hero:
  name: Croupier
  text: 游戏服务器 Sidecar 与运维工具集
  tagline: 只读观测、崩溃产物枚举、Supervisor 状态、OpenWrt 兜底探活——跨平台通用能力
  image:
    src: /logo.svg
    alt: Croupier
  actions:
    - theme: brand
      text: 快速开始
      link: /guide
    - theme: alt
      text: 架构
      link: /architecture
    - theme: alt
      text: GitHub
      link: https://github.com/cuihairu/croupier

features:
  - title: 只读诊断
    details: agent 侧只看不管理：进程/Supervisor 快照、systemd-coredump·WER·DiagnosticReports 三平台崩溃产物枚举，零提权。
  - title: 侧车集成
    details: sidecar 外部函数适配插件：函数/schema 声明映射游戏 HTTP/TCP/管理接口，权限沿用 game/env scope 边界。
  - title: 运维面板
    details: Dashboard·API·SDK 三面齐备，审计与文档一致性有专项核查（见审计-文档一致性页）。
---
