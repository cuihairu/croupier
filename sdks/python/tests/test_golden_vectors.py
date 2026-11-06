"""CR-FIX-05 跨 SDK 行为级契约 golden 向量测试（Python SDK 面）。

读取 proto/contracts/golden_vectors.json（语言中立字节向量），断言本 SDK 的
new_message 产出与向量 frame 逐字节一致，且 parse_message 能还原出
(version, msg_id, req_id, body)。任一 wire 语义漂移即失败。
"""

import json
from pathlib import Path

from croupier.protocol import (
    VERSION_1,
    MSG_REGISTER_REQUEST,
    MSG_HEARTBEAT_REQUEST,
    MSG_TASK_EVENT,
    new_message,
    parse_message,
)

REPO = Path(__file__).resolve().parents[2]
if not (REPO / "proto").exists():
    # 测试从仓库根或子目录运行：兜底向上查找 proto/。
    for cand in Path(__file__).resolve().parents:
        if (cand / "proto" / "contracts").exists():
            REPO = cand
            break
VECTORS = json.loads((REPO / "proto" / "contracts" / "golden_vectors.json").read_text())


def _h(s: str) -> bytes:
    return bytes.fromhex(s)


def test_golden_vectors_wire_frames():
    for name, vec in VECTORS["vectors"].items():
        body = _h(vec["body"])
        want_frame = _h(vec["frame"])
        msg_id = int(vec["msgId"], 16)
        req_id = int(vec["reqId"], 16)

        got = new_message(msg_id, req_id, body)
        assert got == want_frame, f"{name}: frame mismatch got={got.hex()} want={want_frame.hex()}"

        version, got_msg_id, got_req_id, got_body = parse_message(want_frame)
        assert version == VERSION_1, f"{name}: version {version}"
        assert got_msg_id == msg_id, f"{name}: msgId 0x{got_msg_id:06x} != 0x{msg_id:06x}"
        assert got_req_id == req_id, f"{name}: reqId {got_req_id} != {req_id}"
        assert got_body == body, f"{name}: body mismatch"


def test_golden_vectors_msg_ids_are_known():
    """向量使用的 msgId 必须映射到本 SDK 已知常量（契约面不漂移）。"""
    v = VECTORS["vectors"]
    assert int(v["register_request"]["msgId"], 16) == MSG_REGISTER_REQUEST
    assert int(v["heartbeat_request"]["msgId"], 16) == MSG_HEARTBEAT_REQUEST
    assert int(v["task_event"]["msgId"], 16) == MSG_TASK_EVENT
