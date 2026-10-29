#!/usr/bin/env python3
"""Generate the source-reviewed Redis analytics topology in both languages."""
from pathlib import Path
import runpy

BASE = Path(__file__).resolve().parent
SVG = runpy.run_path(str(BASE / 'generate-architecture.py'))['SVG']
OUT = BASE.parents[1] / 'docs/public/architecture'


def topology(lang):
    zh = lang == 'zh'
    def t(cn, en):
        return cn if zh else en
    s = SVG(1200, 1800, t('CROUPIER · 分析链路实际拓扑', 'CROUPIER · ANALYTICS RUNTIME TOPOLOGY'))
    # This overview has no sequence lifelines; override the shared description.
    s.p[0] = s.p[0].replace('<svg ', '<svg data-check-edges="true" ', 1).replace('2026-10-29 source wiring. Time runs down sequence lifelines; ALT alternatives are exclusive. Crossings without dots do not join.', '2026-10-29 source review. Independent processes, in-process modules, stores and external clients. Arrows show writes or query requests, not a timeline. Crossings without dots do not join.')
    s.text(30, 78, t('现状 · 默认 Redis 路径 · 箭头不代表时间顺序', 'Current implementation · Default Redis path · Arrows are not a timeline'), 'small')
    s.box(40, 115, 550, 100, t('外部事件生产方', 'External event producers'), [t('游戏服 / 受信采集端 · 事件与支付 JSON', 'Game servers / trusted collectors · event/payment JSON'), t('HMAC 密钥留在受信端，不分发到玩家客户端', 'Keep HMAC secrets on trusted producers')], 'external')
    s.edge('M315 215 V315', 'external')
    s.text(340, 253, 'HTTP POST /api/ingest/{events,payments}', 'small')
    s.text(340, 281, t('HMAC-SHA256 · timestamp / nonce / body', 'HMAC-SHA256 · timestamp / nonce / body'), 'small')
    s.box(40, 315, 550, 100, 'INGEST · cmd/ingest', [t('独立进程 · 验签 / 限流 / 校验 / 发布', 'Process · authenticate / rate-limit / validate / publish'), t('HTTP 202 只确认接入，不确认仓库落盘', 'HTTP 202 acknowledges ingestion, not warehouse durability')])
    s.edge('M315 415 V525', 'data')
    s.text(340, 455, 'Redis RESP / XADD', 'small')
    s.text(340, 483, t('JSON envelope · data 字段', 'JSON envelope · data field'), 'small')
    s.box(40, 525, 550, 115, 'REDIS · Streams', ['analytics:events / analytics:payments', t('外部数据服务 · 消费组 / pending / 死信流', 'Data service · consumer groups / pending / dead letters'), t('连接凭据与 TLS 由 Redis URL 配置', 'Credentials / TLS depend on the Redis URL')], 'store')
    s.edge('M315 640 V745', 'data', both=True)
    s.text(340, 678, 'RESP / XREADGROUP + XACK', 'small')
    s.text(340, 706, t('worker 主动拉取；返回事件 / 支付', 'Worker polls; events / payments returned'), 'small')
    s.box(40, 745, 550, 130, 'WORKER · cmd/analytics-worker', [t('独立进程 · 消费 / 规范化 / 批写 / 聚合', 'Process · consume / normalize / batch / aggregate'), t('pending 回收 / 失败重试 / 死信', 'Pending reclaim / retry / dead letters'), t('ACK 与批次持久化缺口：ARCH-011/012', 'ACK / batch durability gaps: ARCH-011/012')])
    s.edge('M315 875 V995', 'data')
    s.text(340, 920, 'clickhouse-go / INSERT batches', 'small')
    s.text(340, 948, t('DSN 决定地址 / 认证 / TLS', 'DSN controls endpoint / credentials / TLS'), 'small')
    s.box(40, 995, 550, 130, 'CLICKHOUSE', ['analytics.events / analytics.payments', 'minute_online / daily_users / daily_revenue', t('外部数据服务 · 明细 + 现有聚合表', 'Data service · detail + existing aggregate tables')], 'store')
    s.edge('M315 1245 V1125', 'control', both=True)
    s.text(340, 1167, 'clickhouse-go / SQL SELECT / rows', 'small')
    s.text(340, 1195, t('由 Server 发起 · 仓库只读 API', 'Server initiates · read-only warehouse API'), 'small')
    s.box(40, 1245, 600, 210, 'SERVER · croupier-server', [t('一个独立进程，以下均为进程内模块', 'One process; the following are in-process modules'), t('Analytics API：scope / 报表查询 / 响应', 'Analytics API: scope / report queries / responses'), t('GORM：关系模型查询，与仓库来源不同', 'GORM: relational queries, separate from the warehouse'), t('Telemetry bridge：可选进程内事件发布器', 'Telemetry bridge: optional in-process event publisher'), t('bridge 遥测不等于玩家活跃或完整留存', 'Bridge telemetry is not player activity or retention')], 'module')
    s.edge('M640 1400 H685 V582 H590', 'data', optional=True)
    s.text(745, 560, t('可选 bridge → Redis', 'Optional bridge → Redis'), 'head')
    s.text(745, 595, 'RESP / XADD · analytics:events', 'small')
    s.text(745, 625, t('进程内缓冲 / 后台批次', 'In-process buffer / background batches'), 'small')
    s.text(745, 655, t('无 ingest HTTP / HMAC 跳转', 'No ingest HTTP / HMAC hop'), 'small')
    s.text(745, 1200, t('由 Server 发起 · SQL / GORM', 'Server initiates · SQL / GORM'), 'small')
    s.text(745, 1228, t('请求 / 行结果 · 数据源配置凭据', 'Request / rows · datasource credentials'), 'small')
    s.box(760, 1285, 400, 130, t('关系数据库', 'RELATIONAL DATABASE'), [t('元数据库 / 按 game 路由的库', 'Metadata / per-game routed databases'), t('玩家 / 支付 / cohort 等模型', 'Player / payment / cohort models'), t('不是从 ClickHouse 自动同步', 'No automatic ClickHouse synchronization')], 'store')
    s.edge('M640 1335 H760', 'control', both=True)
    s.edge('M315 1560 V1455', 'control', both=True)
    s.text(340, 1494, 'HTTP / JWT / gameId + env', 'small')
    s.text(340, 1522, t('浏览器请求 Server API，返回报表 JSON', 'Browser calls Server API; report JSON returned'), 'small')
    s.box(40, 1560, 600, 100, t('运营浏览器 · Analytics 页面', 'OPERATOR BROWSER · Analytics pages'), [t('仓库页与关系报表页共用 Server API 入口', 'Warehouse and relational pages use the Server API'), t('浏览器不直连数据库，不持有数据库凭据', 'No browser-to-database connection or DB credentials')], 'external')
    s.text(30, 1707, t('图例：绿色＝写入/事件传输；蓝色＝查询请求与响应；棕色＝外部上报；虚线＝可选。', 'Legend: green = writes/events; blue = query/response; brown = external ingest; dashed = optional.'), 'small')
    s.text(30, 1737, t('交叉线无圆点不相连。节点底色区分进程、进程内模块、存储与外部端。', 'Crossings without dots do not join. Node fills distinguish processes, modules, stores and external clients.'), 'small')
    s.text(30, 1767, t('专业分析集成为目标方案，未画成已部署服务；指标正确性与持久化缺口见 DATA / ARCH 待办。', 'Professional analytics is a target, not a deployed service; see DATA / ARCH tasks for metric and durability gaps.'), 'small')
    s.save(f'analytics-{lang}.svg')


if __name__ == '__main__':
    OUT.mkdir(parents=True, exist_ok=True)
    for language in ('zh', 'en'):
        topology(language)
