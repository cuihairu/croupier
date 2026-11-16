# GUX-001 部署版本与取证（2026-11-17）

目标栈：192.168.5.5 自托管部署（demo 测试环境，非生产）。取证过程不记录任何凭据。

## 部署对账

| 组件 | 镜像 | digest（前 12 位） |
| --- | --- | --- |
| server ×2 | `ghcr.io/cuihairu/croupier-server:main` | `77ab343d4e69` |
| dashboard | `ghcr.io/cuihairu/croupier-dashboard:main` | `038936b3567a` |
| agent ×2 + demo-agent | `ghcr.io/cuihairu/croupier-agent:main`（demo 栈以 `croupier-demo-agent:local` 重标签） | `1d328681704f` |
| ingestion | `ghcr.io/cuihairu/croupier-ingest:main` | `c3583aa05e6e` |
| analytics worker | `ghcr.io/cuihairu/croupier-analytics-worker:main` | `d329dd4632c8` |
| demo-game | `ghcr.io/cuihairu/croupier-demo-game:main`（demo 栈以 `croupier-demo-game:local` 重标签） | 镜像 ID `1b10f8b48c91` |

- 平台栈镜像由 `docker.yml` run `43928190705`（commit `4e2b12c68`）构建；取证时 main 已推进至 `76018b0dc`（其后两 commit 为 docs/compose 参数化，不影响已部署镜像内容）。
- demo-game/demo scope 存在于 games 目录（REST `GET /api/v1/games` 复证可见，envs=[demo]）。存量库上该 scope 由 DB 直插建成（REST 手建会把 gameId 归一成下划线，GAME-013）。
- provider 注册面：server 日志 `Agent registered game_id=demo-game functions=34 warnings=0`；functions API 按头过滤可见 34 个（另有 2 条 game_id 为空的 9/10 月遗留行混入列表，4862 同族既有问题，与本栈无关）。
- exactly-once 重放演练（API 直驱 invoke）：玩家 20001 余额 6532 → 充值 648 → 7180；`docker restart docker-demo-game-1` 后同 operationId 重放 → 回执一致、余额不动；同 operationId 换 payload → 指纹冲突拒绝。

## 截图索引（1600×900，Playwright 实测部署站）

| 文件 | 内容 |
| --- | --- |
| `01-console-home.png` | Runtime Console 首页，scope=Demo Game/DEMO；7 个菜单节点，`0 pages published`，全部真实空态（No pages in this menu yet） |
| `02-console-catalog.png` | `/console/player`（Player Workspace）目录页，同真实空态 |
| `03-player-template.png` | player-manage 模板页（26 行、分页、行操作 View/充值/邮件/编辑/封禁/Delete） |
| `04-player-template` 无独立文件；`04-player-detail.png` | 模板页行详情抽屉（Details：玩家 20001、等级/VIP/状态/余额/最近登录/注册时间） |
| `console-links.txt` | 首页菜单/目录链接清单（7 个 category） |

## 边界（如实）

1. 部署栈当前 **0 pages published**：console 目录/菜单为真实空态；player-manage 模板页渲染的是**浏览器内存预览数据**（余额列 977×n 等差数列、时间戳随挂钟走），不是 demo-game 活数据——demo-game 活数据下 20001 余额为 7180（演练后）。发布页 binding 执行链的活数据验收由 `scripts/e2e/demo-game.sh`（CI 栈）与上述 API 重放演练（部署栈）覆盖。
2. 模板页/发布页的数据源边界即 game-ops-demo.md 的「browser-memory 留作模板预览 fixture；持久 owner 是 demo-game」定位。
3. 已确认测试 scope 为 `demo-game` / `demo`；空态形态、模板边界（五模板入口、player-manage 唯一细化模板）如截图。
