"""
Game Demo - 19 functions matching the Go SDK demo.

Covers: player/order lifecycle actions, leaderboard, inventory, and mail.
Run: cd sdks/python && uv run python examples/game_demo.py
"""

from __future__ import annotations

import json
import os
import signal
import sys
import threading
import time
from dataclasses import dataclass, field
from typing import Any, Callable

from croupier import ClientConfig, CroupierClient, FunctionDescriptor


# ==================== Data Models ====================

@dataclass
class PlayerRecord:
    id: str = ""
    name: str = ""
    level: int = 1
    vip: int = 0
    gold: int = 0
    status: str = "active"
    server: str = "s1"
    createdAt: str = ""
    updatedAt: str = ""
    lastLoginAt: str = ""
    profile: dict[str, Any] | None = None

    def to_dict(self) -> dict[str, Any]:
        d: dict[str, Any] = {
            "id": self.id, "name": self.name, "level": self.level,
            "vip": self.vip, "gold": self.gold, "status": self.status,
            "server": self.server, "createdAt": self.createdAt,
            "updatedAt": self.updatedAt, "lastLoginAt": self.lastLoginAt,
        }
        if self.profile:
            d["profile"] = self.profile
        return d


@dataclass
class OrderRecord:
    id: str = ""
    playerId: str = ""
    productId: str = ""
    amount: int = 0
    currency: str = "CNY"
    status: str = "created"
    channel: str = "gm"
    createdAt: str = ""
    updatedAt: str = ""
    attributes: dict[str, Any] | None = None

    def to_dict(self) -> dict[str, Any]:
        d: dict[str, Any] = {
            "id": self.id, "playerId": self.playerId, "productId": self.productId,
            "amount": self.amount, "currency": self.currency, "status": self.status,
            "channel": self.channel, "createdAt": self.createdAt, "updatedAt": self.updatedAt,
        }
        if self.attributes:
            d["attributes"] = self.attributes
        return d


@dataclass
class LeaderboardEntry:
    playerId: str = ""
    playerName: str = ""
    score: int = 0
    rank: int = 0
    updatedAt: str = ""

    def to_dict(self) -> dict[str, Any]:
        return {
            "playerId": self.playerId, "playerName": self.playerName,
            "score": self.score, "rank": self.rank, "updatedAt": self.updatedAt,
        }


@dataclass
class ItemRecord:
    id: str = ""
    templateId: str = ""
    name: str = ""
    quantity: int = 0
    rarity: str = "common"
    updatedAt: str = ""

    def to_dict(self) -> dict[str, Any]:
        return {
            "id": self.id, "templateId": self.templateId, "name": self.name,
            "quantity": self.quantity, "rarity": self.rarity, "updatedAt": self.updatedAt,
        }


@dataclass
class MailRecord:
    id: str = ""
    playerId: str = ""
    title: str = ""
    content: str = ""
    status: str = "unread"
    reward: dict[str, Any] | None = None
    sentAt: str = ""
    updatedAt: str = ""
    expireAt: str = ""

    def to_dict(self) -> dict[str, Any]:
        d: dict[str, Any] = {
            "id": self.id, "playerId": self.playerId, "title": self.title,
            "content": self.content, "status": self.status,
            "sentAt": self.sentAt, "updatedAt": self.updatedAt,
        }
        if self.reward:
            d["reward"] = self.reward
        if self.expireAt:
            d["expireAt"] = self.expireAt
        return d


# ==================== In-Memory Store ====================

class DemoStore:
    def __init__(self) -> None:
        now = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
        self._lock = threading.Lock()
        self._player_seq = 1002
        self._order_seq = 3002
        self._mail_seq = 5002

        self.players: dict[str, PlayerRecord] = {
            "player_1001": PlayerRecord(
                id="player_1001", name="Alice", level=35, vip=3, gold=128800,
                status="active", server="s1", createdAt=now, updatedAt=now,
                lastLoginAt=now, profile={"guild": "星海旅团", "country": "CN", "platform": "ios"},
            ),
            "player_1002": PlayerRecord(
                id="player_1002", name="Bob", level=42, vip=5, gold=256000,
                status="active", server="s2", createdAt=now, updatedAt=now,
                lastLoginAt=now, profile={"guild": "苍穹守卫", "country": "US", "platform": "android"},
            ),
        }
        self.orders: dict[str, OrderRecord] = {
            "order_3001": OrderRecord(
                id="order_3001", playerId="player_1001", productId="com.croupier.gems.648",
                amount=6480, currency="CNY", status="paid", channel="appstore",
                createdAt=now, updatedAt=now, attributes={"region": "cn"},
            ),
            "order_3002": OrderRecord(
                id="order_3002", playerId="player_1002", productId="battle.pass.s2",
                amount=68, currency="USD", status="pending", channel="googleplay",
                createdAt=now, updatedAt=now,
            ),
        }
        self.leaderboard: dict[str, LeaderboardEntry] = {
            "player_1002": LeaderboardEntry(playerId="player_1002", playerName="Bob", score=98500, rank=1, updatedAt=now),
            "player_1001": LeaderboardEntry(playerId="player_1001", playerName="Alice", score=91200, rank=2, updatedAt=now),
        }
        self.inventories: dict[str, dict[str, ItemRecord]] = {
            "player_1001": {
                "gold_coin": ItemRecord(id="item_gold_coin", templateId="gold_coin", name="金币", quantity=128800, rarity="common", updatedAt=now),
                "hero_ticket": ItemRecord(id="item_hero_ticket", templateId="hero_ticket", name="英雄招募券", quantity=12, rarity="rare", updatedAt=now),
            },
        }
        self.mails: dict[str, list[MailRecord]] = {
            "player_1001": [
                MailRecord(id="mail_5001", playerId="player_1001", title="开服奖励",
                           content="欢迎来到 Croupier Demo World", status="unread",
                           reward={"gold": 10000, "item": "hero_ticket"},
                           sentAt=now, updatedAt=now),
            ],
        }

    def _now(self) -> str:
        return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())

    def _next_player_id(self) -> str:
        self._player_seq += 1
        return f"player_{self._player_seq}"

    def _next_order_id(self) -> str:
        self._order_seq += 1
        return f"order_{self._order_seq}"

    def _next_mail_id(self) -> str:
        self._mail_seq += 1
        return f"mail_{self._mail_seq}"


# ==================== Helpers ====================

def _parse(payload: bytes) -> dict[str, Any]:
    if not payload:
        return {}
    return json.loads(payload.decode("utf-8"))


def _resp(data: dict[str, Any]) -> str:
    data["timestamp"] = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    return json.dumps(data, ensure_ascii=False)


def _str(body: dict, *keys: str) -> str:
    for k in keys:
        v = body.get(k)
        if isinstance(v, str) and v.strip():
            return v.strip()
    return ""


def _int(body: dict, default: int, *keys: str) -> int:
    for k in keys:
        v = body.get(k)
        if isinstance(v, (int, float)):
            return int(v)
        if isinstance(v, str):
            try:
                return int(v.strip())
            except ValueError:
                pass
    return default


def _map(body: dict, key: str) -> dict[str, Any] | None:
    v = body.get(key)
    return v if isinstance(v, dict) else None


def _non_empty(*vals: str) -> str:
    for v in vals:
        if v and v.strip():
            return v.strip()
    return ""


# ==================== Handler Factories ====================

def make_player_create(store: DemoStore) -> Callable:
    def handler(_ctx: str, payload: bytes) -> str:
        body = _parse(payload)
        with store._lock:
            pid = _str(body, "id", "playerId") or store._next_player_id()
            now = store._now()
            r = PlayerRecord(
                id=pid, name=_non_empty(_str(body, "name"), f"Player-{pid}"),
                level=_int(body, 1, "level"), vip=_int(body, 0, "vip"),
                gold=_int(body, 0, "gold"), status=_non_empty(_str(body, "status"), "active"),
                server=_non_empty(_str(body, "server"), "s1"),
                createdAt=now, updatedAt=now, lastLoginAt=now,
                profile=_map(body, "profile"),
            )
            store.players[pid] = r
        return _resp({"status": "success", "action": "player.create", "player": r.to_dict()})
    return handler


def make_player_get(store: DemoStore) -> Callable:
    def handler(_ctx: str, payload: bytes) -> str:
        body = _parse(payload)
        pid = _str(body, "playerId", "id")
        with store._lock:
            r = store.players.get(pid)
        if not r:
            return _resp({"status": "not_found", "message": "player not found"})
        return _resp({"status": "success", "action": "player.get", "player": r.to_dict()})
    return handler


def make_player_update(store: DemoStore) -> Callable:
    def handler(_ctx: str, payload: bytes) -> str:
        body = _parse(payload)
        pid = _str(body, "playerId", "id")
        with store._lock:
            r = store.players.get(pid)
            if not r:
                return _resp({"status": "not_found", "message": "player not found"})
            name = _str(body, "name")
            if name:
                r.name = name
            if "level" in body:
                r.level = _int(body, r.level, "level")
            if "vip" in body:
                r.vip = _int(body, r.vip, "vip")
            if "gold" in body:
                r.gold = _int(body, int(r.gold), "gold")
            status = _str(body, "status")
            if status:
                r.status = status
            server = _str(body, "server")
            if server:
                r.server = server
            profile = _map(body, "profile")
            if profile:
                r.profile = profile
            r.updatedAt = store._now()
        return _resp({"status": "success", "action": "player.update", "player": r.to_dict()})
    return handler


def make_player_delete(store: DemoStore) -> Callable:
    def handler(_ctx: str, payload: bytes) -> str:
        body = _parse(payload)
        pid = _str(body, "playerId", "id")
        with store._lock:
            store.players.pop(pid, None)
            store.inventories.pop(pid, None)
            store.mails.pop(pid, None)
            store.leaderboard.pop(pid, None)
        return _resp({"status": "success", "action": "player.delete", "playerId": pid})
    return handler


def make_player_list(store: DemoStore) -> Callable:
    def handler(_ctx: str, _payload: bytes) -> str:
        with store._lock:
            items = [r.to_dict() for r in sorted(store.players.values(), key=lambda p: p.id)]
        return _resp({"status": "success", "action": "player.list", "items": items, "total": len(items)})
    return handler


def make_order_create(store: DemoStore) -> Callable:
    def handler(_ctx: str, payload: bytes) -> str:
        body = _parse(payload)
        with store._lock:
            oid = _str(body, "orderId", "id") or store._next_order_id()
            now = store._now()
            r = OrderRecord(
                id=oid, playerId=_str(body, "playerId"),
                productId=_non_empty(_str(body, "productId"), "product.demo"),
                amount=_int(body, 0, "amount"), currency=_non_empty(_str(body, "currency"), "CNY"),
                status=_non_empty(_str(body, "status"), "created"),
                channel=_non_empty(_str(body, "channel"), "gm"),
                createdAt=now, updatedAt=now, attributes=_map(body, "attributes"),
            )
            store.orders[oid] = r
        return _resp({"status": "success", "action": "order.create", "order": r.to_dict()})
    return handler


def make_order_get(store: DemoStore) -> Callable:
    def handler(_ctx: str, payload: bytes) -> str:
        body = _parse(payload)
        oid = _str(body, "orderId", "id")
        with store._lock:
            r = store.orders.get(oid)
        if not r:
            return _resp({"status": "not_found", "message": "order not found"})
        return _resp({"status": "success", "action": "order.get", "order": r.to_dict()})
    return handler


def make_order_update(store: DemoStore) -> Callable:
    def handler(_ctx: str, payload: bytes) -> str:
        body = _parse(payload)
        oid = _str(body, "orderId", "id")
        with store._lock:
            r = store.orders.get(oid)
            if not r:
                return _resp({"status": "not_found", "message": "order not found"})
            status = _str(body, "status")
            if status:
                r.status = status
            channel = _str(body, "channel")
            if channel:
                r.channel = channel
            if "amount" in body:
                r.amount = _int(body, int(r.amount), "amount")
            attrs = _map(body, "attributes")
            if attrs:
                r.attributes = attrs
            r.updatedAt = store._now()
        return _resp({"status": "success", "action": "order.update", "order": r.to_dict()})
    return handler


def make_order_delete(store: DemoStore) -> Callable:
    def handler(_ctx: str, payload: bytes) -> str:
        body = _parse(payload)
        oid = _str(body, "orderId", "id")
        with store._lock:
            store.orders.pop(oid, None)
        return _resp({"status": "success", "action": "order.delete", "orderId": oid})
    return handler


def make_order_list(store: DemoStore) -> Callable:
    def handler(_ctx: str, payload: bytes) -> str:
        body = _parse(payload)
        playerId = _str(body, "playerId")
        with store._lock:
            items = [
                r.to_dict() for r in sorted(store.orders.values(), key=lambda o: o.id)
                if not playerId or r.playerId == playerId
            ]
        return _resp({"status": "success", "action": "order.list", "items": items, "total": len(items)})
    return handler


def make_leaderboard_list(store: DemoStore) -> Callable:
    def handler(_ctx: str, _payload: bytes) -> str:
        with store._lock:
            entries = sorted(store.leaderboard.values(), key=lambda e: -e.score)
            items = []
            for i, e in enumerate(entries):
                e.rank = i + 1
                items.append(e.to_dict())
        return _resp({"status": "success", "action": "leaderboard.list", "items": items, "total": len(items)})
    return handler


def make_leaderboard_upsert(store: DemoStore) -> Callable:
    def handler(_ctx: str, payload: bytes) -> str:
        body = _parse(payload)
        pid = _str(body, "playerId")
        if not pid:
            raise ValueError("playerId is required")
        with store._lock:
            playerName = pid
            p = store.players.get(pid)
            if p and p.name:
                playerName = p.name
            e = LeaderboardEntry(playerId=pid, playerName=playerName,
                                 score=_int(body, 0, "score"), updatedAt=store._now())
            store.leaderboard[pid] = e
        return _resp({"status": "success", "action": "leaderboard.upsert", "entry": e.to_dict()})
    return handler


def make_leaderboard_reset(store: DemoStore) -> Callable:
    def handler(_ctx: str, _payload: bytes) -> str:
        with store._lock:
            store.leaderboard.clear()
        return _resp({"status": "success", "action": "leaderboard.reset"})
    return handler


def make_inventory_list(store: DemoStore) -> Callable:
    def handler(_ctx: str, payload: bytes) -> str:
        body = _parse(payload)
        pid = _str(body, "playerId")
        if not pid:
            raise ValueError("playerId is required")
        with store._lock:
            inv = store.inventories.get(pid, {})
            items = [r.to_dict() for r in sorted(inv.values(), key=lambda i: i.templateId)]
        return _resp({"status": "success", "action": "inventory.list", "playerId": pid, "items": items})
    return handler


def make_inventory_grant(store: DemoStore) -> Callable:
    def handler(_ctx: str, payload: bytes) -> str:
        body = _parse(payload)
        pid = _str(body, "playerId")
        tid = _str(body, "templateId", "itemId")
        if not pid or not tid:
            raise ValueError("playerId and templateId are required")
        with store._lock:
            inv = store.inventories.setdefault(pid, {})
            r = inv.get(tid)
            if not r:
                r = ItemRecord(
                    id=f"item_{tid}", templateId=tid,
                    name=_non_empty(_str(body, "name"), tid),
                    rarity=_non_empty(_str(body, "rarity"), "common"),
                )
                inv[tid] = r
            r.quantity += _int(body, 1, "quantity")
            r.updatedAt = store._now()
        return _resp({"status": "success", "action": "inventory.grant", "playerId": pid, "item": r.to_dict()})
    return handler


def make_inventory_consume(store: DemoStore) -> Callable:
    def handler(_ctx: str, payload: bytes) -> str:
        body = _parse(payload)
        pid = _str(body, "playerId")
        tid = _str(body, "templateId", "itemId")
        qty = _int(body, 1, "quantity")
        if not pid or not tid:
            raise ValueError("playerId and templateId are required")
        with store._lock:
            inv = store.inventories.get(pid, {})
            r = inv.get(tid)
            if not r:
                return _resp({"status": "not_found", "message": "item not found"})
            if r.quantity < qty:
                return _resp({"status": "failed", "message": "insufficient quantity", "item": r.to_dict()})
            r.quantity -= qty
            r.updatedAt = store._now()
        return _resp({"status": "success", "action": "inventory.consume", "playerId": pid, "item": r.to_dict()})
    return handler


def make_mail_send(store: DemoStore) -> Callable:
    def handler(_ctx: str, payload: bytes) -> str:
        body = _parse(payload)
        pid = _str(body, "playerId")
        if not pid:
            raise ValueError("playerId is required")
        with store._lock:
            now = store._now()
            r = MailRecord(
                id=store._next_mail_id(), playerId=pid,
                title=_non_empty(_str(body, "title"), "系统邮件"),
                content=_non_empty(_str(body, "content"), "请查收奖励"),
                status="unread", reward=_map(body, "reward"),
                sentAt=now, updatedAt=now, expireAt=_str(body, "expireAt"),
            )
            store.mails.setdefault(pid, []).append(r)
        return _resp({"status": "success", "action": "mail.send", "mail": r.to_dict()})
    return handler


def make_mail_list(store: DemoStore) -> Callable:
    def handler(_ctx: str, payload: bytes) -> str:
        body = _parse(payload)
        pid = _str(body, "playerId")
        if not pid:
            raise ValueError("playerId is required")
        with store._lock:
            items = [m.to_dict() for m in store.mails.get(pid, [])]
        return _resp({"status": "success", "action": "mail.list", "playerId": pid, "items": items, "total": len(items)})
    return handler


def make_mail_claim(store: DemoStore) -> Callable:
    def handler(_ctx: str, payload: bytes) -> str:
        body = _parse(payload)
        pid = _str(body, "playerId")
        mid = _str(body, "mailId", "id")
        if not pid or not mid:
            raise ValueError("playerId and mail_id are required")
        with store._lock:
            for m in store.mails.get(pid, []):
                if m.id == mid:
                    m.status = "claimed"
                    m.updatedAt = store._now()
                    return _resp({"status": "success", "action": "mail.claim", "mail": m.to_dict()})
        return _resp({"status": "not_found", "message": "mail not found"})
    return handler


# Schemas describe the handlers' real wire contract with camelCase JSON
# keys. snake_case is only allowed inside databases, never on the wire.
def _obj(props: dict[str, Any], required: list[str] | None = None) -> dict[str, Any]:
    schema: dict[str, Any] = {"type": "object", "properties": props}
    if required:
        schema["required"] = required
    return schema


def _s() -> dict[str, Any]:
    return {"type": "string"}


def _i() -> dict[str, Any]:
    return {"type": "integer"}


PLAYER_FIELDS: dict[str, Any] = {
    "id": _s(), "name": _s(), "level": _i(), "vip": _i(),
    "gold": _i(), "status": _s(), "server": _s(), "profile": {"type": "object"},
}
ORDER_FIELDS: dict[str, Any] = {
    "id": _s(), "playerId": _s(), "productId": _s(), "amount": _i(),
    "currency": _s(), "status": _s(), "channel": _s(), "attributes": {"type": "object"},
}
PAGINATION: dict[str, Any] = {"page": _i(), "pageSize": _i()}
LIST_OUTPUT: dict[str, Any] = _obj({
    "items": {"type": "array", "items": {"type": "object"}},
    "total": _i(),
})

SCHEMAS: dict[str, dict[str, dict[str, Any]]] = {
    "player.create": {
        "input": _obj(dict(PLAYER_FIELDS)),
        "output": _obj({"player": {"type": "object"}}),
    },
    "player.get": {
        "input": _obj({"id": _s()}, ["id"]),
        "output": _obj({"player": {"type": "object"}}),
    },
    "player.update": {
        "input": _obj(dict(PLAYER_FIELDS), ["id"]),
        "output": _obj({"player": {"type": "object"}}),
    },
    "player.delete": {
        "input": _obj({"id": _s()}, ["id"]),
        "output": _obj({"playerId": _s()}),
    },
    "player.list": {"input": _obj(dict(PAGINATION)), "output": LIST_OUTPUT},
    "order.create": {
        "input": _obj(dict(ORDER_FIELDS)),
        "output": _obj({"order": {"type": "object"}}),
    },
    "order.get": {
        "input": _obj({"id": _s()}, ["id"]),
        "output": _obj({"order": {"type": "object"}}),
    },
    "order.update": {
        "input": _obj({k: ORDER_FIELDS[k] for k in ("id", "status", "channel", "amount", "attributes")}, ["id"]),
        "output": _obj({"order": {"type": "object"}}),
    },
    "order.delete": {
        "input": _obj({"id": _s()}, ["id"]),
        "output": _obj({"orderId": _s()}),
    },
    "order.list": {
        "input": _obj({"playerId": _s(), **PAGINATION}),
        "output": LIST_OUTPUT,
    },
    "leaderboard.list": {"input": _obj(dict(PAGINATION)), "output": LIST_OUTPUT},
    "leaderboard.upsert": {
        "input": _obj({"playerId": _s(), "score": _i()}, ["playerId"]),
        "output": _obj({"entry": {"type": "object"}}),
    },
    "leaderboard.reset": {"input": _obj({}), "output": _obj({})},
    "inventory.list": {
        "input": _obj({"playerId": _s()}, ["playerId"]),
        "output": _obj({"items": {"type": "array", "items": {"type": "object"}}}),
    },
    "inventory.grant": {
        "input": _obj({"playerId": _s(), "templateId": _s(), "quantity": _i()}, ["playerId", "templateId"]),
        "output": _obj({"item": {"type": "object"}}),
    },
    "inventory.consume": {
        "input": _obj({"playerId": _s(), "templateId": _s(), "quantity": _i()}, ["playerId", "templateId"]),
        "output": _obj({"item": {"type": "object"}}),
    },
    "mail.send": {
        "input": _obj({"playerId": _s(), "title": _s(), "content": _s(), "reward": {"type": "object"}, "expireAt": _s()}, ["playerId"]),
        "output": _obj({"mail": {"type": "object"}}),
    },
    "mail.list": {
        "input": _obj({"playerId": _s()}, ["playerId"]),
        "output": LIST_OUTPUT,
    },
    "mail.claim": {
        "input": _obj({"playerId": _s(), "mailId": _s()}, ["playerId", "mailId"]),
        "output": _obj({"mail": {"type": "object"}}),
    },
}


def enrich_descriptor(desc: FunctionDescriptor) -> FunctionDescriptor:
    if not desc.tags:
        desc.tags = [value for value in (desc.resource, desc.operation) if value]
    if not desc.summary:
        desc.summary = f"{desc.resource or 'function'} {desc.operation or 'invoke'}"
    if not desc.description:
        desc.description = (
            f"Demo function {desc.id} for {desc.resource or 'unscoped'} {desc.operation or 'invoke'} operations."
        )
    schemas = SCHEMAS.get(desc.id)
    if schemas:
        if not desc.input_schema:
            desc.input_schema = schemas["input"]
        if not desc.output_schema:
            desc.output_schema = schemas["output"]
        return desc
    if not desc.input_schema:
        desc.input_schema = input_schema_for(desc.resource or "object", desc.operation or "invoke")
    if not desc.output_schema:
        desc.output_schema = {
            "type": "object",
            "properties": {
                "status": {"type": "string"},
                "action": {"type": "string"},
            },
        }
    return desc


def input_schema_for(resource: str, operation: str) -> dict[str, Any]:
    id_key = "playerId" if resource == "inventory" else f"{resource}_id"
    if operation == "create":
        return {
            "type": "object",
            "properties": {id_key: {"type": "string"}, "data": {"type": "object"}},
        }
    if operation == "update":
        return {
            "type": "object",
            "properties": {id_key: {"type": "string"}, "patch": {"type": "object"}},
            "required": [id_key],
        }
    if operation == "delete":
        return {
            "type": "object",
            "properties": {id_key: {"type": "string"}},
            "required": [id_key],
        }
    return {"type": "object", "properties": {id_key: {"type": "string"}}}


# ==================== Main ====================

def main() -> None:
    agent_addr = os.environ.get("CROUPIER_AGENT_ADDR", "127.0.0.1:19091")
    game_id = os.environ.get("CROUPIER_GAME_ID", "demo-game")
    service_id = os.environ.get("CROUPIER_SERVICE_ID", "game-demo-service")
    env_name = os.environ.get("CROUPIER_ENV", "development")

    config = ClientConfig(
        agent_addr=agent_addr,
        game_id=game_id,
        env=env_name,
        service_id=service_id,
        service_version="1.0.0",
    )
    client = CroupierClient(config)
    store = DemoStore()

    fns = [
        ("player.create", "player", "medium", "create", make_player_create(store)),
        ("player.get", "player", "low", "get", make_player_get(store)),
        ("player.update", "player", "medium", "update", make_player_update(store)),
        ("player.delete", "player", "high", "delete", make_player_delete(store)),
        ("player.list", "player", "low", "list", make_player_list(store)),
        ("order.create", "order", "medium", "create", make_order_create(store)),
        ("order.get", "order", "low", "get", make_order_get(store)),
        ("order.update", "order", "medium", "update", make_order_update(store)),
        ("order.delete", "order", "high", "delete", make_order_delete(store)),
        ("order.list", "order", "low", "list", make_order_list(store)),
        ("leaderboard.list", "leaderboard", "low", "list", make_leaderboard_list(store)),
        ("leaderboard.upsert", "leaderboard", "medium", "upsert", make_leaderboard_upsert(store)),
        ("leaderboard.reset", "leaderboard", "high", "reset", make_leaderboard_reset(store)),
        ("inventory.list", "inventory", "low", "list", make_inventory_list(store)),
        ("inventory.grant", "inventory", "medium", "grant", make_inventory_grant(store)),
        ("inventory.consume", "inventory", "medium", "consume", make_inventory_consume(store)),
        ("mail.send", "mail", "medium", "send", make_mail_send(store)),
        ("mail.list", "mail", "low", "list", make_mail_list(store)),
        ("mail.claim", "mail", "medium", "claim", make_mail_claim(store)),
    ]

    for fid, resource, risk, operation, handler in fns:
        desc = enrich_descriptor(FunctionDescriptor(
            id=fid, version="1.0.0", resource=resource, risk=risk,
            operation=operation,
        ))
        client.register_function(desc, handler)
        print(f"  registered: {fid}")

    print(f"\nstarting game demo: agent={agent_addr} game={game_id} env={env_name} service={service_id}")
    client.connect()
    print("connected to agent, press Ctrl+C to stop\n")

    def _shutdown(_sig: int, _frame: Any) -> None:
        print("\nstopping...")
        client.disconnect()
        sys.exit(0)

    signal.signal(signal.SIGINT, _shutdown)
    signal.signal(signal.SIGTERM, _shutdown)

    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        _shutdown(0, None)


if __name__ == "__main__":
    main()
