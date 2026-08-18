/**
 * Game Demo - 19 functions matching the Go SDK demo.
 *
 * Covers: player/order lifecycle actions, leaderboard, inventory, and mail.
 * Run: cd sdks/js && pnpm ts-node examples/game_demo.ts
 */

import { createClient, FunctionDescriptor, FunctionHandler, CroupierClient } from "../src";

// ==================== Data Models ====================

interface PlayerRecord {
  id: string; name: string; level: number; vip: number; gold: number;
  status: string; server: string; createdAt: string; updatedAt: string;
  last_login_at: string; profile?: Record<string, unknown>;
}

interface OrderRecord {
  id: string; playerId: string; productId: string; amount: number;
  currency: string; status: string; channel: string; createdAt: string;
  updatedAt: string; attributes?: Record<string, unknown>;
}

interface LeaderboardEntry {
  playerId: string; playerName: string; score: number; rank: number; updatedAt: string;
}

interface ItemRecord {
  id: string; templateId: string; name: string; quantity: number; rarity: string; updatedAt: string;
}

interface MailRecord {
  id: string; playerId: string; title: string; content: string; status: string;
  reward?: Record<string, unknown>; sentAt: string; updatedAt: string; expireAt?: string;
}

// ==================== In-Memory Store ====================

class DemoStore {
  playerSeq = 1002;
  orderSeq = 3002;
  mailSeq = 5002;

  players: Map<string, PlayerRecord> = new Map();
  orders: Map<string, OrderRecord> = new Map();
  leaderboard: Map<string, LeaderboardEntry> = new Map();
  inventories: Map<string, Map<string, ItemRecord>> = new Map();
  mails: Map<string, MailRecord[]> = new Map();

  constructor() {
    const now = new Date().toISOString();

    this.players.set("player_1001", {
      id: "player_1001", name: "Alice", level: 35, vip: 3, gold: 128800,
      status: "active", server: "s1", createdAt: now, updatedAt: now,
      last_login_at: now, profile: { guild: "星海旅团", country: "CN", platform: "ios" },
    });
    this.players.set("player_1002", {
      id: "player_1002", name: "Bob", level: 42, vip: 5, gold: 256000,
      status: "active", server: "s2", createdAt: now, updatedAt: now,
      last_login_at: now, profile: { guild: "苍穹守卫", country: "US", platform: "android" },
    });

    this.orders.set("order_3001", {
      id: "order_3001", playerId: "player_1001", productId: "com.croupier.gems.648",
      amount: 6480, currency: "CNY", status: "paid", channel: "appstore",
      createdAt: now, updatedAt: now, attributes: { region: "cn" },
    });
    this.orders.set("order_3002", {
      id: "order_3002", playerId: "player_1002", productId: "battle.pass.s2",
      amount: 68, currency: "USD", status: "pending", channel: "googleplay",
      createdAt: now, updatedAt: now,
    });

    this.leaderboard.set("player_1002", { playerId: "player_1002", playerName: "Bob", score: 98500, rank: 1, updatedAt: now });
    this.leaderboard.set("player_1001", { playerId: "player_1001", playerName: "Alice", score: 91200, rank: 2, updatedAt: now });

    const inv = new Map<string, ItemRecord>();
    inv.set("gold_coin", { id: "item_gold_coin", templateId: "gold_coin", name: "金币", quantity: 128800, rarity: "common", updatedAt: now });
    inv.set("hero_ticket", { id: "item_hero_ticket", templateId: "hero_ticket", name: "英雄招募券", quantity: 12, rarity: "rare", updatedAt: now });
    this.inventories.set("player_1001", inv);

    this.mails.set("player_1001", [{
      id: "mail_5001", playerId: "player_1001", title: "开服奖励",
      content: "欢迎来到 Croupier Demo World", status: "unread",
      reward: { gold: 10000, item: "hero_ticket" }, sentAt: now, updatedAt: now,
    }]);
  }

  now(): string { return new Date().toISOString(); }
  nextPlayerId(): string { return `player_${++this.playerSeq}`; }
  nextOrderId(): string { return `order_${++this.orderSeq}`; }
  nextMailId(): string { return `mail_${++this.mailSeq}`; }
}

// ==================== Helpers ====================

function parsePayload(payload: string): Record<string, unknown> {
  if (!payload) return {};
  try { return JSON.parse(payload); } catch { return {}; }
}

function resp(data: Record<string, unknown>): string {
  data.timestamp = new Date().toISOString();
  return JSON.stringify(data);
}

function str(body: Record<string, unknown>, ...keys: string[]): string {
  for (const k of keys) {
    const v = body[k];
    if (typeof v === "string" && v.trim()) return v.trim();
  }
  return "";
}

function num(body: Record<string, unknown>, def: number, ...keys: string[]): number {
  for (const k of keys) {
    const v = body[k];
    if (typeof v === "number") return Math.floor(v);
    if (typeof v === "string") { const n = parseInt(v, 10); if (!isNaN(n)) return n; }
  }
  return def;
}

function nonEmpty(...vals: string[]): string {
  for (const v of vals) if (v && v.trim()) return v.trim();
  return "";
}

// ==================== Handler Factories ====================

function playerCreate(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const id = str(body, "id", "playerId") || store.nextPlayerId();
    const now = store.now();
    const rec: PlayerRecord = {
      id, name: nonEmpty(str(body, "name"), `Player-${id}`),
      level: num(body, 1, "level"), vip: num(body, 0, "vip"),
      gold: num(body, 0, "gold"), status: nonEmpty(str(body, "status"), "active"),
      server: nonEmpty(str(body, "server"), "s1"),
      createdAt: now, updatedAt: now, last_login_at: now,
      profile: body.profile as Record<string, unknown> | undefined,
    };
    store.players.set(id, rec);
    return resp({ status: "success", action: "player.create", player: rec });
  };
}

function playerGet(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const r = store.players.get(str(body, "playerId", "id"));
    if (!r) return resp({ status: "not_found", message: "player not found" });
    return resp({ status: "success", action: "player.get", player: r });
  };
}

function playerUpdate(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const r = store.players.get(str(body, "playerId", "id"));
    if (!r) return resp({ status: "not_found", message: "player not found" });
    const name = str(body, "name"); if (name) r.name = name;
    if ("level" in body) r.level = num(body, r.level, "level");
    if ("vip" in body) r.vip = num(body, r.vip, "vip");
    if ("gold" in body) r.gold = num(body, r.gold, "gold");
    const status = str(body, "status"); if (status) r.status = status;
    const server = str(body, "server"); if (server) r.server = server;
    if (body.profile && typeof body.profile === "object") r.profile = body.profile as Record<string, unknown>;
    r.updatedAt = store.now();
    return resp({ status: "success", action: "player.update", player: r });
  };
}

function playerDelete(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const id = str(body, "playerId", "id");
    store.players.delete(id); store.inventories.delete(id);
    store.mails.delete(id); store.leaderboard.delete(id);
    return resp({ status: "success", action: "player.delete", playerId: id });
  };
}

function playerList(store: DemoStore): FunctionHandler {
  return async () => {
    const items = [...store.players.values()].sort((a, b) => a.id.localeCompare(b.id));
    return resp({ status: "success", action: "player.list", items, total: items.length });
  };
}

function orderCreate(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const id = str(body, "orderId", "id") || store.nextOrderId();
    const now = store.now();
    const rec: OrderRecord = {
      id, playerId: str(body, "playerId"),
      productId: nonEmpty(str(body, "productId"), "product.demo"),
      amount: num(body, 0, "amount"), currency: nonEmpty(str(body, "currency"), "CNY"),
      status: nonEmpty(str(body, "status"), "created"),
      channel: nonEmpty(str(body, "channel"), "gm"),
      createdAt: now, updatedAt: now,
      attributes: body.attributes as Record<string, unknown> | undefined,
    };
    store.orders.set(id, rec);
    return resp({ status: "success", action: "order.create", order: rec });
  };
}

function orderGet(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const r = store.orders.get(str(body, "orderId", "id"));
    if (!r) return resp({ status: "not_found", message: "order not found" });
    return resp({ status: "success", action: "order.get", order: r });
  };
}

function orderUpdate(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const r = store.orders.get(str(body, "orderId", "id"));
    if (!r) return resp({ status: "not_found", message: "order not found" });
    const status = str(body, "status"); if (status) r.status = status;
    const channel = str(body, "channel"); if (channel) r.channel = channel;
    if ("amount" in body) r.amount = num(body, r.amount, "amount");
    if (body.attributes && typeof body.attributes === "object") r.attributes = body.attributes as Record<string, unknown>;
    r.updatedAt = store.now();
    return resp({ status: "success", action: "order.update", order: r });
  };
}

function orderDelete(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const id = str(body, "orderId", "id");
    store.orders.delete(id);
    return resp({ status: "success", action: "order.delete", orderId: id });
  };
}

function orderList(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const pid = str(body, "playerId");
    const items = [...store.orders.values()]
      .filter(o => !pid || o.playerId === pid)
      .sort((a, b) => a.id.localeCompare(b.id));
    return resp({ status: "success", action: "order.list", items, total: items.length });
  };
}

function leaderboardList(store: DemoStore): FunctionHandler {
  return async () => {
    const sorted = [...store.leaderboard.values()].sort((a, b) => b.score - a.score);
    sorted.forEach((e, i) => { e.rank = i + 1; });
    return resp({ status: "success", action: "leaderboard.list", items: sorted, total: sorted.length });
  };
}

function leaderboardUpsert(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const pid = str(body, "playerId");
    if (!pid) throw new Error("playerId is required");
    const p = store.players.get(pid);
    const entry: LeaderboardEntry = {
      playerId: pid, playerName: p?.name || pid,
      score: num(body, 0, "score"), rank: 0, updatedAt: store.now(),
    };
    store.leaderboard.set(pid, entry);
    return resp({ status: "success", action: "leaderboard.upsert", entry });
  };
}

function leaderboardReset(store: DemoStore): FunctionHandler {
  return async () => {
    store.leaderboard.clear();
    return resp({ status: "success", action: "leaderboard.reset" });
  };
}

function inventoryList(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const pid = str(body, "playerId");
    if (!pid) throw new Error("playerId is required");
    const inv = store.inventories.get(pid) || new Map();
    const items = [...inv.values()].sort((a, b) => a.templateId.localeCompare(b.templateId));
    return resp({ status: "success", action: "inventory.list", playerId: pid, items });
  };
}

function inventoryGrant(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const pid = str(body, "playerId");
    const tid = str(body, "templateId", "itemId");
    if (!pid || !tid) throw new Error("playerId and templateId are required");
    if (!store.inventories.has(pid)) store.inventories.set(pid, new Map());
    const inv = store.inventories.get(pid)!;
    let r = inv.get(tid);
    if (!r) {
      r = { id: `item_${tid}`, templateId: tid, name: nonEmpty(str(body, "name"), tid),
            quantity: 0, rarity: nonEmpty(str(body, "rarity"), "common"), updatedAt: "" };
      inv.set(tid, r);
    }
    r.quantity += num(body, 1, "quantity");
    r.updatedAt = store.now();
    return resp({ status: "success", action: "inventory.grant", playerId: pid, item: r });
  };
}

function inventoryConsume(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const pid = str(body, "playerId");
    const tid = str(body, "templateId", "itemId");
    const qty = num(body, 1, "quantity");
    if (!pid || !tid) throw new Error("playerId and templateId are required");
    const inv = store.inventories.get(pid);
    const r = inv?.get(tid);
    if (!r) return resp({ status: "not_found", message: "item not found" });
    if (r.quantity < qty) return resp({ status: "failed", message: "insufficient quantity", item: r });
    r.quantity -= qty;
    r.updatedAt = store.now();
    return resp({ status: "success", action: "inventory.consume", playerId: pid, item: r });
  };
}

function mailSend(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const pid = str(body, "playerId");
    if (!pid) throw new Error("playerId is required");
    const now = store.now();
    const rec: MailRecord = {
      id: store.nextMailId(), playerId: pid,
      title: nonEmpty(str(body, "title"), "系统邮件"),
      content: nonEmpty(str(body, "content"), "请查收奖励"),
      status: "unread", reward: body.reward as Record<string, unknown> | undefined,
      sentAt: now, updatedAt: now, expireAt: str(body, "expireAt") || undefined,
    };
    if (!store.mails.has(pid)) store.mails.set(pid, []);
    store.mails.get(pid)!.push(rec);
    return resp({ status: "success", action: "mail.send", mail: rec });
  };
}

function mailList(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const pid = str(body, "playerId");
    if (!pid) throw new Error("playerId is required");
    const items = store.mails.get(pid) || [];
    return resp({ status: "success", action: "mail.list", playerId: pid, items, total: items.length });
  };
}

function mailClaim(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const pid = str(body, "playerId");
    const mid = str(body, "mailId", "id");
    if (!pid || !mid) throw new Error("playerId and mailId are required");
    const list = store.mails.get(pid) || [];
    const m = list.find(x => x.id === mid);
    if (!m) return resp({ status: "not_found", message: "mail not found" });
    m.status = "claimed"; m.updatedAt = store.now();
    return resp({ status: "success", action: "mail.claim", mail: m });
  };
}

function enrichDescriptor(desc: FunctionDescriptor): FunctionDescriptor {
  const tags = desc.tags || ([desc.resource, desc.operation].filter(Boolean) as string[]);
  return {
    ...desc,
    tags,
    summary: desc.summary || `${desc.resource || "function"} ${desc.operation || "invoke"}`,
    description:
      desc.description ||
      `Demo function ${desc.id} for ${desc.resource || "unscoped"} ${desc.operation || "invoke"} operations.`,
    operation_id: desc.operation_id || desc.id,
    input_schema: desc.input_schema || inputSchemaFor(desc.resource || "object", desc.operation || "invoke"),
    output_schema: desc.output_schema || {
      type: "object",
      properties: {
        status: { type: "string" },
        action: { type: "string" },
      },
    },
  };
}

function inputSchemaFor(resource: string, operation: string): Record<string, unknown> {
  const idKey = resource === "inventory" ? "playerId" : `${resource}_id`;
  if (operation === "create") {
    return {
      type: "object",
      properties: { [idKey]: { type: "string" }, data: { type: "object" } },
    };
  }
  if (operation === "update") {
    return {
      type: "object",
      properties: { [idKey]: { type: "string" }, patch: { type: "object" } },
      required: [idKey],
    };
  }
  if (operation === "delete") {
    return {
      type: "object",
      properties: { [idKey]: { type: "string" } },
      required: [idKey],
    };
  }
  return { type: "object", properties: { [idKey]: { type: "string" } } };
}

// ==================== Main ====================

async function main(): Promise<void> {
  const agentAddr = process.env.CROUPIER_AGENT_ADDR || "127.0.0.1:19091";
  const gameId = process.env.CROUPIER_GAME_ID || "demo-game";
  const serviceId = process.env.CROUPIER_SERVICE_ID || "game-demo-service";
  const envName = process.env.CROUPIER_ENV || "development";

  const client = createClient({
    agentAddr,
    gameId,
    env: envName,
    serviceId,
    serviceVersion: "1.0.0",
    insecure: true,
    timeout: 30000,
  });

  const store = new DemoStore();

  const fns: Array<[string, string, string, string, FunctionHandler]> = [
    ["player.create", "player", "medium", "create", playerCreate(store)],
    ["player.get", "player", "low", "get", playerGet(store)],
    ["player.update", "player", "medium", "update", playerUpdate(store)],
    ["player.delete", "player", "high", "delete", playerDelete(store)],
    ["player.list", "player", "low", "list", playerList(store)],
    ["order.create", "order", "medium", "create", orderCreate(store)],
    ["order.get", "order", "low", "get", orderGet(store)],
    ["order.update", "order", "medium", "update", orderUpdate(store)],
    ["order.delete", "order", "high", "delete", orderDelete(store)],
    ["order.list", "order", "low", "list", orderList(store)],
    ["leaderboard.list", "leaderboard", "low", "list", leaderboardList(store)],
    ["leaderboard.upsert", "leaderboard", "medium", "upsert", leaderboardUpsert(store)],
    ["leaderboard.reset", "leaderboard", "high", "reset", leaderboardReset(store)],
    ["inventory.list", "inventory", "low", "list", inventoryList(store)],
    ["inventory.grant", "inventory", "medium", "grant", inventoryGrant(store)],
    ["inventory.consume", "inventory", "medium", "consume", inventoryConsume(store)],
    ["mail.send", "mail", "medium", "send", mailSend(store)],
    ["mail.list", "mail", "low", "list", mailList(store)],
    ["mail.claim", "mail", "medium", "claim", mailClaim(store)],
  ];

  for (const [id, resource, risk, operation, handler] of fns) {
    const desc = enrichDescriptor({
      id, version: "1.0.0", resource, risk, operation,
    });
    client.registerFunction(desc, handler);
    console.log(`  registered: ${id}`);
  }

  console.log(`\nstarting game demo: agent=${agentAddr} game=${gameId} env=${envName} service=${serviceId}`);

  const shutdown = () => {
    console.log("\nstopping...");
    client.disconnect();
    process.exit(0);
  };
  process.on("SIGINT", shutdown);
  process.on("SIGTERM", shutdown);

  try {
    await client.connect();
    console.log("connected to agent, press Ctrl+C to stop\n");
    await new Promise<void>((resolve) => {
      process.on("SIGINT", resolve);
      process.on("SIGTERM", resolve);
    });
  } catch (err) {
    console.error("failed:", err);
    process.exit(1);
  }
}

main().catch((err) => {
  console.error("unhandled error:", err);
  process.exit(1);
});
