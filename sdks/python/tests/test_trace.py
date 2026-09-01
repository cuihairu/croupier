"""OTel trace 传播辅助（一期）：context JSON 提取语义。"""

import json

from croupier.trace import trace_id_from_context, trace_parent_from_context


def test_extract_trace_fields():
    ctx = json.dumps({"traceparent": "00-abc-def-01", "trace_id": "abc", "game_id": "demo"})
    assert trace_parent_from_context(ctx) == "00-abc-def-01"
    assert trace_id_from_context(ctx) == "abc"


def test_absent_fields_yield_empty():
    ctx = json.dumps({"game_id": "demo"})
    assert trace_parent_from_context(ctx) == ""
    assert trace_id_from_context(ctx) == ""


def test_malformed_context_yields_empty():
    assert trace_parent_from_context("") == ""
    assert trace_id_from_context("not json") == ""
    assert trace_parent_from_context("[1,2]") == ""


def test_non_string_value_yields_empty():
    ctx = json.dumps({"trace_id": 123})
    assert trace_id_from_context(ctx) == ""
