#!/usr/bin/env python3
"""Source-reviewed bilingual topology/lifecycle SVGs; standard library only."""
from pathlib import Path
from html import escape
import shutil
OUT=Path(__file__).resolve().parents[2]/'docs/public/architecture'
class SVG:
 def __init__(self,w,h,title):
  self.w,self.h=w,h
  self.p=[f'<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="0 0 {w} {h}" role="img" aria-labelledby="title desc"><title id="title">{escape(title)}</title><desc id="desc">2026-10-30 source review. Time runs down sequence lifelines; ALT alternatives are exclusive. Crossings without dots do not join.</desc><defs>']
  for k,c in [('control','#53658d'),('data','#237b72'),('external','#976a3d')]:self.p.append(f'<marker id="{k}" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0L10 5L0 10Z" fill="{c}"/></marker>')
  self.p.append('''</defs><style>text{font:17px Arial,'Noto Sans CJK SC','Microsoft YaHei',sans-serif;fill:#263344}.title{font-size:28px;font-weight:700}.head{font-size:21px;font-weight:700}.small{font-size:15px;fill:#526174}.edge{stroke-width:2;fill:none}.control{stroke:#53658d}.data{stroke:#237b72}.external{stroke:#976a3d}.optional{stroke-dasharray:7 5}.life{stroke:#bdc6d2;stroke-width:1.3;stroke-dasharray:5 6}.message{paint-order:stroke;stroke:#f8fafc;stroke-width:6px;stroke-linejoin:round}</style>''')
  self.p.append(f'<rect width="{w}" height="{h}" fill="#f8fafc"/>');self.text(30,45,title,'title')
 def text(self,x,y,s,cls='',anchor='start'):self.p.append(f'<text x="{x}" y="{y}" class="{cls}" text-anchor="{anchor}">{escape(s)}</text>')
 def box(self,x,y,w,h,title,lines=(),kind='process'):
  color={'process':'#fff','module':'#eef1fb','store':'#eaf5f1','external':'#fff4e8','gap':'#f6eeee'}[kind]
  self.p.append(f'<g data-node="{escape(title)}"><rect x="{x}" y="{y}" width="{w}" height="{h}" rx="10" fill="{color}" stroke="#aebdce"/>');self.text(x+16,y+30,title,'head')
  for i,t in enumerate(lines):self.text(x+16,y+58+i*25,t)
  self.p.append('</g>')
 def edge(self,d,kind='control',optional=False,both=False):self.p.append(f'<path d="{d}" class="edge {kind}{" optional" if optional else ""}" marker-end="url(#{kind})"'+(f' marker-start="url(#{kind})"' if both else '')+'/>')
 def save(self,name):(OUT/name).write_text('\n'.join(self.p+['</svg>'])+'\n')
def topology(lang):
 z=lang=='zh'
 def t(a,b):return a if z else b
 s=SVG(1200,2310,t('CROUPIER · 当前运行拓扑','CROUPIER · CURRENT RUNTIME TOPOLOGY'))
 s.text(30,77,t('2026-10-30 · 竖向总览 · Server 内文字是进程内模块','2026-10-30 · Vertical overview · Text inside Server identifies modules'),'small')
 s.box(365,110,470,100,t('运营客户端','Operator clients'),[t('浏览器 Web / Flutter 移动端','Web browser / Flutter mobile'),t('页面、受控操作、任务查询','Pages, governed actions, task queries')])
 s.edge('M600 210 V295','data',both=True);s.text(620,242,'HTTP :18780 / JWT / SSE','small');s.text(620,266,t('可选 nginx L7 代理','Optional nginx L7 proxy'),'small')
 s.box(310,295,580,300,'SERVER · '+t('一个模块化控制面进程','One modular control plane'),[t('身份 / scope / 政策 / 审批 / 审计','Identity / scope / policy / approvals / audit'),t('契约 → 页面发布 → binding 执行','Contracts → publication → binding execution'),'registry / functioninvoke / dispatch / Ops',t('任务状态与 SSE / 进程内 scheduler','Task state and SSE / in-process scheduler'),t('活连接归本实例；审批当前为内存','Live sessions local; approvals currently in memory'),t('本机探针 / 通知出口 / telemetry bridge','Local probes / outlets / telemetry bridge'),t('core/ 是公共包，不是额外服务','core/ packages are not separate services')],'module')
 s.box(15,320,265,125,t('其他 Server 实例','Other Server instances'),['TCP interconnect',t('单跳转发 / epoch','One-hop forward / epoch'),t('明文；mTLS 是目标','Plain TCP; mTLS is a target')]);s.edge('M310 390 H280',both=True)
 s.box(925,295,260,125,t('关系数据库','Relational DB'),[t('元数据 / 页面 / 任务','Metadata / pages / tasks'),t('契约 / 审计 / 业务','Contracts / audit / business'),t('可选 game DB 路由','Optional game DB routing')],'store');s.edge('M890 350 H925','data',both=True)
 s.box(925,465,260,105,t('共享协调后端','Shared coordination'),[t('DB 或 Redis：租约','DB or Redis: leases'),'membership / owner'],'store');s.edge('M890 510 H925',optional=True,both=True)
 s.box(15,485,265,110,t('可选外部集成','Optional integrations'),['Herald / OIDC / Vault',t('平台 API / 观测后端','Platform APIs / telemetry')],'external');s.edge('M310 540 H280','external',optional=True,both=True)
 s.edge('M600 765 V595');s.text(620,638,t('Agent 主动建连 :19090（可经 HAProxy L4）','Agents dial :19090 (optional HAProxy L4)'),'small');s.text(620,665,t('TCP + protobuf · TLS 按配置 · session 双向','TCP + protobuf · configured TLS · bidirectional'),'small');s.text(620,692,t('注册 / 心跳 / 配置 / 命令 / 上报','Register / heartbeat / config / commands / reports'),'small')
 s.text(30,735,t('四类独立 Agent','Four Agent roles'),'small');s.text(620,735,t('每实例一个 gameId + env；圆点为汇接点','One gameId + env each; dots join the bus'),'small')
 s.edge('M285 795 V620 H320 V595',optional=True);s.text(25,650,t('可选 HTTP 扩展同步','Optional HTTP extension pull'),'small');s.text(25,675,t('认证链未接通，见正文','Auth wiring gap; see text'),'small')
 s.p.append('<path d="M150 795 V765 H1050 V795" class="edge control"/>')
 for x in (450,750):s.p.append(f'<path d="M{x} 795 V765" class="edge control"/>')
 for x in (150,450,600,750,1050):s.p.append(f'<circle cx="{x}" cy="765" r="4" fill="#53658d"/>')
 for x,name,lines in [(15,'sidecar',[t('SDK 网关 / 函数 / 任务','SDK gateway / functions / tasks'),t('受管程序 / 探针 / 扩展','Managed processes / extensions'),'Provider :19091']),(315,'capture',[t('CDC / 位点 / 过滤规则','CDC / checkpoints / rules'),t('本地 spool / 可选 journal','Local spool / optional journal'),t('告警请求有 ACK','Alert requests have ACKs')]),(615,'synthetics',[t('目标探测 / 结果上报','Target probes / result reports'),t('外部探测插件宿主','External probe plugin host'),t('按角色装配探测能力','Role-specific probe wiring')]),(915,'devops',[t('注册 / 白名单配置同步','Registration / allowlist sync'),t('SSH Broker / Hub 基础','SSH Broker / Hub foundations'),t('面板中继未接通','Console relay incomplete')])]:s.box(x,795,270,145,name,lines)
 s.box(15,1060,270,125,t('游戏进程 + SDK 库','Game process + SDK'),[t('主动建连；双向调用','Dials; bidirectional calls'),t('游戏权威数据在游戏侧','Game owns authoritative data'),t('摘流已接入；待发任务有缺口','Drain wired; queued-task gap')]);s.edge('M275 1060 V940','data');s.text(25,990,'TCP / protobuf :19091','small');s.text(25,1015,'Connect / Invoke / result','small')
 s.box(315,1060,270,125,t('游戏源数据库','Game source databases'),['MySQL / MariaDB binlog','PostgreSQL pgoutput','MongoDB change stream'],'external');s.edge('M570 940 V1060','external',both=True);s.text(325,990,t('复制连接 / 事件返回','Replication / event stream'),'small');s.text(325,1015,t('源凭据 / 本地位点','Source credentials / bookmark'),'small')
 s.box(615,1060,270,125,t('探测目标','Probe targets'),['HTTP / TCP / protocol',t('按配置的目标与凭据','Configured targets / credentials'),t('Server/sidecar 亦可探测','Server/sidecar can also probe')],'external');s.edge('M870 940 V1060','external',both=True);s.text(625,990,t('轮询请求 / 阶段结果','Poll / phase results'),'small')
 s.box(915,1060,270,125,t('SSH 主机','SSH hosts'),[t('本地 Broker 调用才进入','Local Broker calls only'),'Credentials / known_hosts',t('RDP / VNC 未实现','RDP / VNC unavailable')],'external');s.edge('M1170 940 V1060','external',both=True);s.text(925,990,'SSH / allowlist','small');s.text(925,1015,t('不是已通的面板链路','No wired console relay'),'small')
 s.edge('M35 940 H5 V1370 H15',optional=True);s.text(25,1250,t('sidecar 宿主管理插件生命周期','sidecar owns plugin lifecycle'),'small')
 s.box(15,1315,270,125,t('函数插件子进程','Function plugin children'),['go-plugin / local gRPC',t('宿主启动 / 调用 / Kill','Host starts / invokes / kills'),t('可选；不是游戏 SDK','Optional; separate from SDK')])
 s.box(315,1315,270,125,t('受管程序子进程','Managed processes'),[t('sidecar Ops / 本地 cron','sidecar Ops / local cron'),'start / stop / restart / update',t('本机文件与期望状态','Local files / desired state')]);s.edge('M270 940 H300 V1370 H315')
 s.box(615,1315,270,125,t('探测插件子进程','Probe plugin children'),['go-plugin / local gRPC','Describe / Check',t('超时 / 退避 / 熔断','Timeout / backoff / breaker')]);s.edge('M865 940 H900 V1370 H885',optional=True)
 s.edge('M880 595 V610 H1189 V1290 H880 V1185','external',optional=True);s.edge('M280 940 H290 V1270 H800 V1185','external',optional=True);s.text(325,1260,t('sidecar / Server 本机探针 → 同类外部目标','sidecar / Server-local probes → external targets'),'small')
 s.box(915,1315,270,125,t('外部平台 API','External platform APIs'),[t('Server/sidecar 扩展路径','Server/sidecar extension paths'),t('HTTP / 平台凭据','HTTP / platform credentials'),t('按配置启用','Enabled by configuration')],'external');s.edge('M265 940 H295 V1220 H1193 V1375 H1185','external',optional=True);s.text(320,1208,t('可选 sidecar 扩展 → 外部平台（与 devops 无关）','Optional sidecar extension → platform (independent of devops)'),'small')
 s.text(30,1505,t('独立分析管道 · CDC 原始事件不会自动进入此管道','Separate analytics pipeline · raw CDC is not automatically published here'),'head')
 s.box(15,1550,270,100,t('外部事件生产者','Event producers'),[t('事件 / 支付 JSON','Event / payment JSON'),t('与游戏 SDK 函数路由独立','Separate from SDK routing')],'external');s.box(375,1550,350,100,'ingest · HTTP :8088',['HMAC / time window / limits',t('签名接入与 MQ 发布','Signed intake / queue publish')]);s.edge('M285 1600 H375','data');s.text(298,1585,'HTTP','small')
 s.box(375,1740,350,100,'Redis Streams',[t('事件流 / 消费组 / PEL','Streams / consumer groups / PEL'),t('凭据与持久化依配置','Configured credentials / persistence')],'store');s.edge('M550 1650 V1740','data');s.text(570,1703,'XADD','small');s.edge('M890 575 H1197 V1790 H725','data',optional=True);s.text(750,1688,t('Server telemetry bridge（可选）','Server telemetry bridge (optional)'),'small')
 s.box(375,1910,350,100,'analytics-worker',[t('Redis 消费 / 聚合 / 批写','Redis consume / aggregate / batch'),t('内置 worker 不消费 Kafka','Built-in worker is not Kafka-based')]);s.edge('M550 1840 V1910','data');s.text(570,1880,'XREADGROUP / XACK','small');s.box(845,1910,340,100,'ClickHouse',[t('事件 / 支付 / 分析聚合','Events / payments / aggregates'),t('批写确认窗口待修复','Batch / ACK durability gap')],'store');s.edge('M725 1960 H845','data');s.text(735,1945,'native SQL','small')
 s.box(15,2090,1170,160,t('读图规则与现状边界','Legend and current limitations'),[t('蓝线：控制 / 会话；绿线：业务数据；棕线：外部依赖；虚线：可选连接。','Blue: control/session; green: data; brown: external dependency; dashed: optional.'),t('圆点是汇接点；无圆点的交叉线不相连。双箭头表示请求与返回，不表示双方都建连。','Dots join edges; crossings without dots do not. Two heads show request/result, not two dialers.'),t('图是现状，不证明已部署；认证、数据、启动顺序和缺口见 runtime-topology 正文。','Topology is current wiring, not deployment proof. See runtime-topology for trust and lifecycle evidence.'),t('Redis 协调与分析用途分开；共享部署的故障和容量边界待核实。','Redis coordination and analytics are distinct uses; shared placement needs failure/capacity checks.')],'module')
 s.save(f'system-{lang}.svg');shutil.copyfile(OUT/f'system-{lang}.svg',OUT/f'system-{lang}-mobile.svg')
def sequence(lang,name,title,actors,rows):
 def t(v):return v[0] if lang=='zh' else v[1]
 h=260+len(rows)*116;s=SVG(1200,h,t(title));xs=[180,460,740,1020]
 s.text(30,78,t(('时间向下 ↓ · 虚线为生命线 · ALT 框内选一个分支 · 无圆点交叉不相连','Time ↓ · Dashed lifelines · Choose one ALT branch · Crossings do not join')),'small')
 for x,a in zip(xs,actors):s.box(x-115,105,230,65,t(a));s.p.append(f'<path d="M{x} 170 V{h-65}" class="life"/>')
 for i,row in enumerate(rows):
  y=215+i*116
  if row[0] in ('note','alt'):
   if row[0]=='alt' and (i==0 or rows[i-1][0]!='alt'):
    n=1
    while i+n<len(rows) and rows[i+n][0]=='alt':n+=1
    if n>1:
     s.p.append(f'<rect x="40" y="{y-28}" width="1120" height="{n*116-8}" rx="12" fill="#faf5f5" stroke="#976a3d" stroke-width="2"/>')
     for j in range(n-1):s.text(600,y+j*116+87,t(('或 · 仅选一个分支','OR · choose one branch')),'small','middle')
   s.box(50,y-20,1100,85,('ALT · ' if row[0]=='alt' else '')+t(row[1]),[t(row[2])],'gap' if row[0]=='alt' else 'module')
   if len(row)>3:s.edge(f'M{xs[row[3]]} {y+54} H{xs[row[4]]}')
  else:
   _,a,b,label=row;s.text(600,y+4,f'{i+1:02d} · '+t(label),'message','middle')
   s.edge(f'M{xs[a]} {y+22} h55 v24 h-55' if a==b else f'M{xs[a]} {y+30} H{xs[b]}')
 s.text(30,h-25,t(('结束：仅代表图示边界，未承诺的恢复/排空能力见正文','End: guarantees stop at this boundary; see text for recovery/drain gaps')),'small');s.save(f'{name}-{lang}.svg')
# Row records: note/ALT boxes or directed messages; language strings share topology.
A=('Agent 宿主','Agent host');S=('Server / owner','Server / owner');G=('游戏 SDK','Game SDK')
SCENARIOS={
'registration':(('启动注册与断线恢复','Startup, registration and reconnect'),[G,A,S,('数据库 / 协调','DB / coordination')],[
 ('note',('前置条件','Preconditions'),('先校验配置/scope；Server 先装配 DB/resolver/可选 cluster 再监听','Validate configuration/scope; Server wires DB/resolver/optional cluster before listeners')),
 ('msg',1,2,('Agent 主动建 TCP/TLS → Register(scope / functions / Providers)','Agent dials TCP/TLS → Register(scope / functions / Providers)')),
 ('msg',2,3,('校验连接身份绑定并更新 registry / agent_sessions / 可选 owner','Validate session identity; update registry / agent_sessions / optional owner')),
 ('msg',2,1,('Register ACK(sessionId / owner)；失败则关闭连接并重试','Register ACK(sessionId / owner); failure closes connection and retries')),
 ('msg',0,1,('SDK 主动 Connect(serviceId / descriptors)，可晚于 Agent 上线','SDK initiates Connect(serviceId / descriptors), possibly after Agent registration')),
 ('msg',1,0,('Connect ACK(sessionId)；本地 Provider 可路由','Connect ACK(sessionId); local Provider becomes routable')),
 ('msg',1,2,('函数快照变化 → 去抖后重新 Register；不是命令重放','Snapshot changes → debounced Register update; not command replay')),
 ('msg',1,2,('周期 Heartbeat(owner) → Server 续期与 ACK','Periodic Heartbeat(owner) → Server refreshes expiry and acknowledges')),
 ('alt',('连接健康','Connection healthy'),('继续心跳/配置同步；不执行重连分支','Continue heartbeat/config sync; skip the reconnect branch')),
 ('alt',('断链 / 连续心跳失败','Disconnect / repeated heartbeat failure'),('关闭旧连接 → 新连接与全量注册；不恢复在途命令','Close old connection → reconnect/full registration; no in-flight command restoration')),
 ('msg',2,2,('仅断线分支：按 agentID + sessionID 清旧会话，不删除新会话','Disconnect branch only: remove old session conditionally by agentID + sessionID')),
 ('note',('结束条件','Termination'),('Stop / ctx 取消结束心跳重试并关闭连接；SDK 恢复依各语言实现','Stop/context cancellation ends loops and closes connections; SDK recovery varies by language'))]),
'execution':(('受控同步函数执行','Governed synchronous invocation'),[('运营客户端','Operator client'),('Server / 治理','Server / governance'),('sidecar','sidecar'),G],[
 ('note',('前置条件','Preconditions'),('已登录、授权 scope、有效 binding；Agent / Provider 在线','Authenticated scope and valid binding; Agent/Provider online')),
 ('msg',0,1,('HTTP binding execute → 快照、输入契约、政策与权限门禁','HTTP binding execute → snapshot, input-contract, policy and authorization gates')),
 ('alt',('拒绝 / 待审批：本次请求结束','Reject / approval required: request ends'),('返回拒绝或审批 ID，不下发；批准后经续跑重新校验','Return rejection or approval ID, no dispatch; approval continuation revalidates')),
 ('alt',('允许同步执行','Synchronous execution permitted'),('以下仅属允许分支；task 模式见独立任务图','Only the permitted branch continues; task mode has a separate sequence diagram')),
 ('msg',1,2,('本机 session 或单跳 owner → Invoke','Local session or one-hop owner → Invoke')),
 ('msg',2,3,('Provider Invoke(JSON)；插件/内建函数按路由另选','Provider Invoke(JSON); plugin/built-in routes select their own executor')),
 ('msg',3,2,('返回结果 / 错误；超时可能结果未知','Return result/error; timeout can mean unknown outcome')),
 ('msg',2,1,('InvokeResponse / error 经已有会话返回','InvokeResponse / error returns over the existing session')),
 ('msg',1,0,('按路径记录审计 → HTTP 结果 / 错误','Path-dependent audit → HTTP result / error')),
 ('note',('结束条件与幂等边界','Termination and idempotency'),('HTTP 返回不承诺游戏副作用可安全重试；未知结果需查游戏凭据','HTTP completion does not imply safe side-effect retry; unknown outcomes require game receipts'))]),
'task':(('异步任务接收、终态与取消','Asynchronous task acceptance, completion and cancellation'),[('运营客户端','Operator client'),('Server / DB','Server / DB'),('sidecar TaskRunner','sidecar TaskRunner'),G],[
 ('note',('前置条件','Preconditions'),('仅表示已治理的函数/页面 task 入口；裸任务/调度门禁仍有缺口','Governed function/page tasks only; raw task/schedule entry gates are incomplete')),
 ('msg',0,1,('HTTP 任务请求；Server 分配 taskId','HTTP task request; Server allocates taskId')),
 ('msg',1,1,('CreateRun 成功才派发；写失败结束本次请求','Persist CreateRun before dispatch; write failure ends this request')),
 ('msg',1,2,('通过 session / owner 路由 StartTask(taskId / InvokeRequest)','Route StartTask(taskId / InvokeRequest) via session / owner')),
 ('msg',2,1,('StartTask ACK(taskId)：接收，不是执行完成','StartTask ACK(taskId): accepted, not completed')),
 ('msg',1,0,('HTTP taskId；随后查询 / SSE','HTTP taskId; client then queries / subscribes to SSE')),
 ('note',('并发说明','Concurrency note'),('started 与 ACK 可交错；后台执行不以 HTTP 结束为结束','started may race with ACK; background execution outlives the HTTP response')),
 ('msg',2,3,('后台 Provider Invoke；上报 started','Background Provider Invoke; report started')),
 ('alt',('自然结束','Natural completion'),('结果产生 completed / failed 二选一；最后清理 task map / cancel','Result produces completed or failed; defer removes task map entry / cancels context')),
 ('alt',('用户取消，可与自然结束竞争','User cancellation may race with completion'),('先记意图 → CancelTask → cancel ctx；实际结果决定 cancelled，不是回滚','Persist intent → CancelTask → cancel context; actual outcome decides cancelled, without rollback')),
 ('msg',2,1,('终态事件 → task_events / task_runs；取消不回退已终态','Terminal event → task_events / task_runs; cancellation must not regress terminal state')),
 ('msg',1,0,('查询 / SSE 返回已存终态；正常观察结束','Query / SSE returns stored terminal state: normal observation ends')),
 ('note',('实际缺口','Current gap'),('终态有限重试；状态/事件部分写入缺口与跨重启去重尚待收口','Bounded terminal retry; partial-write and cross-restart dedup gaps remain'))]),
'events':(('CDC 告警上报与通知出口','CDC alert reporting and notification outlets'),[('源数据库','Source DB'),('capture','capture'),S,('站内 / Herald','Station / Herald')],[
 ('note',('前置条件','Preconditions'),('源复制权限/bookmark 与已注册上行；Herald 按配置启用','Source replication permissions/bookmark and registered uplink; Herald is opt-in')),
 ('msg',1,0,('加载位点 → Open / Poll；源持续返回 ChangeEvent','Load bookmark → Open / Poll; source streams ChangeEvents back')),
 ('msg',1,1,('可选 journal → consume / 规则 → AckCheckpoint；周期保存位点','Optional journal → consume/rules → AckCheckpoint; periodically save bookmark')),
 ('msg',1,1,('规则命中先持久 spool；入队满仍尝试留盘','Persist rule hits in spool first; queue pressure still attempts durable storage')),
 ('msg',1,2,('AlertEventRequest(rule / position / key / time / scope)','AlertEventRequest(rule / position / key / time / scope)')),
 ('msg',2,2,('连接身份归一化；确定性告警 ID 幂等入库','Normalize session identity; idempotent alert insert with deterministic ID')),
 ('msg',2,3,('入库成功才 Dispatch 异步通知；送达可先于或晚于 ACK','After persistence: Dispatch starts async delivery; delivery may precede or follow ACK')),
 ('alt',('上报成功','Report succeeded'),('CaptureRuleHitAck：Server → capture；删除对应 spool 项','CaptureRuleHitAck: Server → capture; remove the matching spool entry'),2,1),
 ('alt',('上报失败','Report failed'),('保留 spool 并重试；入库失败不通知，ACK 丢失则结果未知','Keep spool and retry; persistence failure skips delivery, while a lost ACK leaves outcome unknown')),
 ('note',('通知失败语义','Notification failure semantics'),('维护抑制停止通知；永久拒绝不重试；网络错有限重试。存储 ACK 不是外部送达','Maintenance suppresses; permanent errors stop; transport errors retry finitely. Storage ACK is not delivery')),
 ('note',('结束条件','Termination'),('源 EOF 刷位点/Close 后重开；ctx 取消最终刷位点并关闭源连接','Source EOF flushes/closes then reopens; cancellation flushes and closes the source'))]),
'plugin':(('本地插件生命周期','Local plugin lifecycle'),[('调用方','Caller'),A,('插件子进程','Plugin child'),('目标 / 结果','Target / result')],[
 ('note',('前置条件','Preconditions'),('插件已配置且未裁剪；本地 go-plugin gRPC，不改变 SDK 协议','Plugin configured and included in build; local go-plugin gRPC is separate from SDK transport')),
 ('msg',0,1,('进入并发 gate；Stop / 预算到期时拒绝','Acquire concurrency gate; reject when stopped or request budget expires')),
 ('msg',1,2,('首次/恢复调用：exec → handshake / Dispense → Describe','First/recovery call: exec → handshake / Dispense → Describe')),
 ('msg',2,1,('Describe ACK(name / version)；失败 Kill 并记退避','Describe ACK(name/version); failure kills child and records backoff')),
 ('msg',1,2,('握手成功才进入有界 Check / Invoke；健康进程复用','Only after handshake: bounded Check / Invoke; reuse healthy children')),
 ('msg',2,3,('插件按其实现访问目标并产生结果','Plugin accesses its target and produces a result')),
 ('msg',2,1,('结果 / 错误；探测凭据脱敏；释放并发 gate','Result/error; redact probe credentials and release concurrency gate')),
 ('alt',('失败 / 超时 / 崩溃','Failure / timeout / crash'),('Kill → 失败计数/退避/熔断；另次允许调用才恢复','Kill → failure count/backoff/breaker; restart on a later eligible call')),
 ('alt',('成功','Success'),('更新健康状态并复用，不执行失败分支','Update health and reuse; skip the failure branch')),
 ('msg',0,1,('Stop：拒绝新工作，最多等 5s 在途，然后 Kill 子进程','Stop: reject new work, wait at most 5s for in-flight calls, then kill children')),
 ('note',('结束条件','Termination'),('宿主清理子进程；握手与凭据过滤不构成安全沙箱','Host cleans up children; handshake and credential filtering do not constitute a sandbox'))]),
'shutdown':(('当前排空与停机边界','Current drain and shutdown boundaries'),[G,('sidecar','sidecar'),S,('后台 / 数据库','Background / DB')],[
 ('note',('独立触发：SDK drain 与 Server SIGTERM 不是同一个协议','Independent triggers: SDK drain and Server SIGTERM are separate'),('上半是 drain 请求，下半是信号停机，不暗示自动联动','Top: drain request; bottom: signal shutdown. No automatic coordination implied')),
 ('msg',0,1,('ProviderDrain(sessionId / reason / retryAfterMs)','ProviderDrain(sessionId / reason / retryAfterMs)')),
 ('msg',1,0,('标记并摘路由；等待 RPC 在途归零或宽限到期后 ACK','Mark/remove routing; ACK when RPC in-flight is zero or grace expires')),
 ('note',('SDK drain 分支到此结束','SDK drain branch ends here'),('不能把 ACK 解释为游戏任务和资源已排空','ACK does not mean game tasks or resources have drained')),
 ('msg',2,2,('独立 SIGTERM：启动 30s 外层截止等待','Independent SIGTERM: start the outer 30s shutdown deadline')),
 ('msg',2,2,('StopAccepting → HTTP drain → rootCancel（不先等活跃会话）','StopAccepting → HTTP drain → rootCancel (do not wait for sessions first)')),
 ('alt',('调度停止与会话等待在预算内结束','Scheduler/session waits finish within the budget'),('StopScheduler / ControlService.Stop → WaitConnections → ServiceContext.Close','StopScheduler / ControlService.Stop → WaitConnections → ServiceContext.Close')),
 ('alt',('外层截止先到','Outer deadline expires first'),('记录超时；返回前 defer StopScheduler 仍可能无界等待（ARCH-017）','Log timeout; final deferred StopScheduler can still block (ARCH-017)')),
 ('msg',2,3,('正常返回还运行 scheduler / cluster / telemetry defer；不是全局事务','Normal return also runs scheduler/cluster/telemetry defers; not a global transaction')),
 ('note',('缺失保证','Missing guarantees'),('Close 已接入；待发任务 drain / 事件部分写入 / 总截止仍有缺口','Close is wired; queued-task drain, partial event writes and total deadline gaps remain'))])}
if __name__=='__main__':
 OUT.mkdir(parents=True,exist_ok=True)
 for lang in ('zh','en'):
  topology(lang)
  for name,(title,actors,rows) in SCENARIOS.items():sequence(lang,name,title,actors,rows)
