"""OTel trace 传播（一期·无依赖版）。

平台在 invoke 的 handler context JSON（即 metadata 序列化）里携带 W3C
``traceparent`` 与冗余明文 ``trace_id``（见
docs/architecture/sdk-otel-propagation.md）。本模块提供读取辅助——游戏方
据此做日志关联，或接入自己的 OTel 体系（SDK 不内置 exporter/自动埋点）。
"""

from __future__ import annotations

import json
from typing import Any, Dict

METADATA_TRACEPARENT = "traceparent"
METADATA_TRACE_ID = "trace_id"


def trace_parent_from_context(context_json: str) -> str:
    """从 handler context JSON 提取 W3C traceparent（无则空串）。"""
    return _field(context_json, METADATA_TRACEPARENT)


def trace_id_from_context(context_json: str) -> str:
    """从 handler context JSON 提取 trace_id 明文（无则空串）。

    可直接拼进游戏方日志/错误信息，去 Jaeger/Grafana 查询整条链路。
    """
    return _field(context_json, METADATA_TRACE_ID)


def _field(context_json: str, key: str) -> str:
    if not context_json:
        return ""
    try:
        parsed: Any = json.loads(context_json)
    except (TypeError, ValueError):
        return ""
    if not isinstance(parsed, dict):
        return ""
    value = parsed.get(key)
    if isinstance(value, str):
        return value.strip()
    return ""


__all__ = [
    "METADATA_TRACEPARENT",
    "METADATA_TRACE_ID",
    "trace_parent_from_context",
    "trace_id_from_context",
]
