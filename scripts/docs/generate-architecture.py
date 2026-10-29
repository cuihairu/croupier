#!/usr/bin/env python3
"""Generate current-source architecture SVGs. No external dependencies."""
from pathlib import Path
from html import escape

OUT = Path(__file__).resolve().parents[2] / 'docs/public/architecture'

def generate(lang, mobile):
    zh = lang == 'zh'
    def tr(c, e): return c if zh else e
    w, h = (640, 1580) if mobile else (1200, 1050)
    parts = [f'''<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="0 0 {w} {h}" role="img" aria-labelledby="title desc">
<title id="title">{tr('Croupier 当前架构', 'Croupier current architecture')}</title>
<desc id="desc">{tr('控制台访问 Server；四类 Agent 出站连接控制面，SDK 接入 sidecar。共享数据库与可选集群协调，独立 Redis 分析管道。', 'Console to Server; four Agent roles connect outbound, SDKs connect to the sidecar. Relational state, optional cluster coordination and a separate Redis analytics pipeline.')}</desc>
<defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0 L10 5 L0 10Z" fill="#7c5260"/></marker></defs>
<style>text{{font-family:Arial,'Noto Sans CJK SC','Microsoft YaHei',sans-serif;fill:#292830}} .heading{{font-size:26px;font-weight:700}} .label{{font-size:20px;font-weight:700}} .body{{font-size:{20 if mobile else 17}px;fill:#55535e}} .note{{font-size:{18 if mobile else 16}px;fill:#6a6269}} .line{{fill:none;stroke:#7c5260;stroke-width:2;marker-end:url(#arrow)}} .dash{{stroke-dasharray:6 5}}</style>
<rect width="{w}" height="{h}" rx="16" fill="#faf8f5"/>
''']
    def text(x,y,s,cls='body'):
        parts.append(f'<text x="{x}" y="{y}" class="{cls}">{escape(s)}</text>')
    def box(x,y,bw,bh,title,lines,accent=False,dashed=False):
        fill,stroke=('#f6e9ed','#b77a8b') if accent else ('#ffffff','#d8d2d5')
        parts.append(f'<rect x="{x}" y="{y}" width="{bw}" height="{bh}" rx="12" fill="{fill}" stroke="{stroke}"'+(' stroke-dasharray="6 5"' if dashed else '')+'/>')
        text(x+20,y+32,title,'label')
        for i,s in enumerate(lines): text(x+20,y+63+i*27,s)
    def line(d,dash=False): parts.append(f'<path d="{d}" class="line'+(' dash' if dash else '')+'"/>')
    text(28,43,'CROUPIER / '+tr('当前运行架构','CURRENT ARCHITECTURE'),'heading')
    text(28,73,tr('源码核对 · 2026-10-28','Source review · 2026-10-28'),'note')
    if not mobile:
        box(30,104,330,105,tr('运营控制台','Operator console'),['React / Umi · Page Studio','HTTP API / SSE'])
        box(415,104,360,105,tr('Web 入口 · 可选 L7','Web ingress · optional L7'),['nginx → Server HTTP :18780',tr('HTTPS 通常在代理终止','HTTPS commonly ends at proxy')])
        box(835,104,335,105,tr('Agent 入口 · 可选 L4','Agent ingress · optional L4'),['HAProxy → control :19090',tr('按长连接分配 Server','Distributes persistent sessions')])
        line('M360 156 H415'); line('M595 209 V265'); line('M1000 209 V240 H750 V265')
        box(30,265,745,260,'SERVER · '+tr('模块化控制面','MODULAR CONTROL PLANE'),[
            tr('HTTP：认证 / scope / 政策 / 页面与契约','HTTP: identity / scope / policy / pages and contracts'),
            tr('执行：functioninvoke → dispatch → Agent session','Execution: functioninvoke → dispatch → Agent session'),
            tr('异步：task_runs / task_events · 查询与 SSE','Async: task_runs / task_events · queries and SSE'),
            tr('可选多实例：owner 转发 · 一跳限制 · epoch fencing','Optional HA: owner forwarding · one hop · epoch fencing'),
            tr('审批当前接内存；互联当前为内网明文 TCP','Approvals wired to memory; interconnect is plaintext TCP'),
            tr('审计按路径/政策记录；TLS 按配置启用','Audit follows path/policy; TLS is configuration-dependent')],True)
        box(835,265,335,126,tr('关系数据库','Relational database'),[tr('业务 / 契约 / 页面 / 任务','Business / contracts / pages / tasks'),tr('SQL 审计 · 可选 game 分库','SQL audit · optional game DBs')])
        box(835,418,335,107,tr('集群协调 · 可选','Cluster coordination · optional'),[tr('共享 DB 或 Redis','Shared DB or Redis'),tr('成员租约 / Agent owner','Membership / Agent owner')],dashed=True)
        line('M775 330 H835'); line('M775 471 H835',True)
        text(30,568,tr('AGENTS / 每实例一个 gameId + env，主动连接 Server','AGENTS / One gameId + env per instance; outbound connections'),'label')
        # Shared bus explicitly describes connection establishment, not invocation direction.
        line('M170 610 V592 H1184 V156 H1170')
        for x in [460,750,1040]: parts.append(f'<path d="M{x} 610 V592" fill="none" stroke="#7c5260" stroke-width="2"/>')
        box(30,610,270,136,'sidecar', [tr('SDK 网关 / 函数 / 任务','SDK gateway / functions / tasks'),'provider :19091',tr('受管程序与函数插件','Managed processes / plugins')])
        box(320,610,270,136,'capture',[tr('数据库 CDC','Database CDC'),tr('位点 / 过滤 / 规则','Checkpoints / filters / rules'),tr('独立采集角色','Separate capture role')])
        box(610,610,270,136,'synthetics',[tr('外部 HTTP 探测','External HTTP probes'),tr('结果 / 探测插件','Results / probe plugins'),tr('独立探测角色','Separate probe role')])
        box(900,610,270,136,'devops',[tr('会话基础 / 白名单','Session foundations / allowlist'),tr('面板中继尚未接通','Console relay incomplete'),tr('RDP / VNC 尚不可用','RDP / VNC unavailable')],dashed=True)
        box(30,792,400,90,tr('游戏进程 + SDK','Game process + SDK'),[tr('主动连接 sidecar；会话内双向调用','Dials sidecar; bidirectional session')]); line('M165 792 V746')
        box(460,792,710,90,tr('公共能力 core/ · 按角色组合','Shared core/ packages · composed by each role'),['transport / register / backoff / configsync / pluginhost / report'])
        text(30,921,tr('独立分析管道 · 不经过 SDK 函数路由','Separate analytics pipeline · outside SDK function routing'),'label')
        box(30,940,250,78,'ingest',[tr('接收事件','Receives events')]);box(330,940,250,78,'Redis Streams',[tr('队列 / 消费组','Queue / consumer groups')]);box(630,940,250,78,'analytics-worker',[tr('消费 / 聚合','Consume / aggregate')]);box(930,940,240,78,'ClickHouse',[tr('分析存储','Analytics storage')])
        for x in [280,580,880]: line(f'M{x} 978 H{x+50}')
    else:
        box(24,100,592,91,tr('运营控制台 · HTTP API / SSE','Operator console · HTTP API / SSE'),['React / Umi · Page Studio / Console'])
        line('M320 191 V222')
        box(24,222,592,116,tr('两类可选入口','Two optional ingress paths'),['L7 nginx → HTTP :18780', 'L4 HAProxy → Agent control :19090'])
        line('M320 338 V370')
        box(24,370,592,255,'SERVER · '+tr('模块化控制面','MODULAR CONTROL PLANE'),[
            tr('认证 / scope / 政策 / 契约 / 页面','Identity / scope / policy / contracts / pages'),
            'functioninvoke → dispatch → Agent session',
            tr('任务状态 / SSE / SQL 审计','Task state / SSE / SQL audit'),
            tr('可选 HA：owner 转发 + epoch fencing','Optional HA: owner forwarding + epoch fencing'),
            tr('审批接内存；实例互联目前为明文','Approvals in memory; plaintext interconnect'),
            tr('TLS 按配置启用，开发模板未开启','TLS configurable; off in development templates')],True)
        line('M320 625 V657',True)
        box(24,657,592,116,tr('持久状态与可选协调','Persistent state and optional coordination'),[tr('关系库：业务 / 页面 / 任务 / 审计','Relational DB: business / pages / tasks / audit'),tr('共享 DB 或 Redis：成员 / owner 租约','Shared DB or Redis: membership / owner leases')])
        text(44,817,tr('Agent 主动连上方 L4 / control，session 双向','Agents dial L4 / control above; sessions are bidirectional'),'note')
        box(24,838,592,200,tr('按角色部署的 Agent','Role-specific Agents'),[
            tr('sidecar：SDK 网关 / 函数 / 任务 / 插件','sidecar: SDK gateway / functions / tasks / plugins'),
            tr('capture：CDC / 位点 / 规则','capture: CDC / checkpoints / rules'),
            tr('synthetics：HTTP 探测 / 探测插件','synthetics: HTTP probes / probe plugins'),
            tr('devops：会话基础已接；面板中继未完成','devops: session foundations; console relay pending'),
            tr('每个实例声明一个 gameId + env','Each instance declares one gameId + env')])
        line('M320 1068 V1038')
        box(24,1068,592,90,tr('游戏进程 + SDK → sidecar :19091','Game process + SDK → sidecar :19091'),[tr('SDK 主动建连，承载双向函数调用','SDK initiates a bidirectional provider session')])
        box(24,1185,592,116,tr('公共运行时 core/ · 按角色装配','Shared core/ runtime · explicit role composition'),['transport / register / backoff / configsync','pluginhost / report / process / healthprobe'])
        box(24,1330,592,174,tr('独立分析管道','Separate analytics pipeline'),['ingest → Redis Streams','→ analytics-worker → ClickHouse',tr('内置 worker 消费 Redis，不消费 Kafka','Built-in worker consumes Redis, not Kafka'),tr('与 SDK 函数执行链独立','Separate from SDK function execution')])
        text(24,1542,tr('箭头标注建连或数据流；完整边界见架构正文','Arrows label connection or data flow; see text for limits'),'note')
    parts.append('</svg>')
    (OUT/f'system-{lang}{"-mobile" if mobile else ""}.svg').write_text('\n'.join(parts), encoding='utf-8')

if __name__ == '__main__':
    OUT.mkdir(parents=True, exist_ok=True)
    for language in ('zh','en'):
        for small in (False, True): generate(language, small)
