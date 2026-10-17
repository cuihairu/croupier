---
title: Design docs
---

# Design docs

This index lists implemented design briefs that explain component boundaries, workflows, and governance mechanisms.

| Area                 | Document                                                                                                                                                             |
| -------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Agent runtime        | [Agent core](./agent-core-design), [agent inventory](./agents-inventory), [supervisor](./agent-supervisor-design).                                                   |
| Capture and CDC      | [Capture agent](./capture-agent-design), [CDC filter policy](./cdc-filter-policy-design).                                                                            |
| Operations workflows | [Batch compensation](./batch-operations-compensation-design), [high-risk operations](./high-risk-operation-contract-design), [incident reports](./incident-reports). |
| Console and pages    | [Dashboard composition](./agent-dashboard-design), [default menus](./default-game-menus), [menu management](./menu-management).                                      |
| Extensibility        | [Plugin mechanism](./plugin-mechanism), [game data sources](./game-data-sources), [cloud resource requests](./cloud-resource-requests).                              |

Every brief states its implementation status and known boundaries. When a design and implementation disagree, update the design before changing the contract.
