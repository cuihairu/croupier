/**
 * Game Demo - 19 functions matching the Go SDK demo.
 *
 * Covers: player CRUD, order CRUD, leaderboard, inventory, mail.
 * Run: cd sdks/js && pnpm ts-node examples/game_demo.ts
 */

import { createClient, FunctionDescriptor, FunctionHandler, CroupierClient } from "../src";

// ==================== Data Models ====================

interface PlayerRecord {
  id: string; name: string; level: number; vip: number; gold: number;
  status: string; server: string; created_at: string; updated_at: string;
  last_login_at: string; profile?: Record<string, unknown>;
}

interface OrderRecord {
  id: string; player_id: string; product_id: string; amount: number;
  currency: string; status: string; channel: string; created_at: string;
  updated_at: string; attributes?: Record<string, unknown>;
}

interface LeaderboardEntry {
  player_id: string; player_name: string; score: number; rank: number; updated_at: string;
}

interface ItemRecord {
  id: string; template_id: string; name: string; quantity: number; rarity: string; updated_at: string;
}

interface MailRecord {
  id: string; player_id: string; title: string; content: string; status: string;
  reward?: Record<string, unknown>; sent_at: string; updated_at: string; expire_at?: string;
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
      status: "active", server: "s1", created_at: now, updated_at: now,
      last_login_at: now, profile: { guild: "星海旅团", country: "CN", platform: "ios" },
    });
    this.players.set("player_1002", {
      id: "player_1002", name: "Bob", level: 42, vip: 5, gold: 256000,
      status: "active", server: "s2", created_at: now, updated_at: now,
      last_login_at: now, profile: { guild: "苍穹守卫", country: "US", platform: "android" },
    });

    this.orders.set("order_3001", {
      id: "order_3001", player_id: "player_1001", product_id: "com.croupier.gems.648",
      amount: 6480, currency: "CNY", status: "paid", channel: "appstore",
      created_at: now, updated_at: now, attributes: { region: "cn" },
    });
    this.orders.set("order_3002", {
      id: "order_3002", player_id: "player_1002", product_id: "battle.pass.s2",
      amount: 68, currency: "USD", status: "pending", channel: "googleplay",
      created_at: now, updated_at: now,
    });

    this.leaderboard.set("player_1002", { player_id: "player_1002", player_name: "Bob", score: 98500, rank: 1, updated_at: now });
    this.leaderboard.set("player_1001", { player_id: "player_1001", player_name: "Alice", score: 91200, rank: 2, updated_at: now });

    const inv = new Map<string, ItemRecord>();
    inv.set("gold_coin", { id: "item_gold_coin", template_id: "gold_coin", name: "金币", quantity: 128800, rarity: "common", updated_at: now });
    inv.set("hero_ticket", { id: "item_hero_ticket", template_id: "hero_ticket", name: "英雄招募券", quantity: 12, rarity: "rare", updated_at: now });
    this.inventories.set("player_1001", inv);

    this.mails.set("player_1001", [{
      id: "mail_5001", player_id: "player_1001", title: "开服奖励",
      content: "欢迎来到 Croupier Demo World", status: "unread",
      reward: { gold: 10000, item: "hero_ticket" }, sent_at: now, updated_at: now,
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
    const id = str(body, "id", "player_id") || store.nextPlayerId();
    const now = store.now();
    const rec: PlayerRecord = {
      id, name: nonEmpty(str(body, "name"), `Player-${id}`),
      level: num(body, 1, "level"), vip: num(body, 0, "vip"),
      gold: num(body, 0, "gold"), status: nonEmpty(str(body, "status"), "active"),
      server: nonEmpty(str(body, "server"), "s1"),
      created_at: now, updated_at: now, last_login_at: now,
      profile: body.profile as Record<string, unknown> | undefined,
    };
    store.players.set(id, rec);
    return resp({ status: "success", action: "player.create", player: rec });
  };
}

function playerGet(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const r = store.players.get(str(body, "player_id", "id"));
    if (!r) return resp({ status: "not_found", message: "player not found" });
    return resp({ status: "success", action: "player.get", player: r });
  };
}

function playerUpdate(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const r = store.players.get(str(body, "player_id", "id"));
    if (!r) return resp({ status: "not_found", message: "player not found" });
    const name = str(body, "name"); if (name) r.name = name;
    if ("level" in body) r.level = num(body, r.level, "level");
    if ("vip" in body) r.vip = num(body, r.vip, "vip");
    if ("gold" in body) r.gold = num(body, r.gold, "gold");
    const status = str(body, "status"); if (status) r.status = status;
    const server = str(body, "server"); if (server) r.server = server;
    if (body.profile && typeof body.profile === "object") r.profile = body.profile as Record<string, unknown>;
    r.updated_at = store.now();
    return resp({ status: "success", action: "player.update", player: r });
  };
}

function playerDelete(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const id = str(body, "player_id", "id");
    store.players.delete(id); store.inventories.delete(id);
    store.mails.delete(id); store.leaderboard.delete(id);
    return resp({ status: "success", action: "player.delete", player_id: id });
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
    const id = str(body, "order_id", "id") || store.nextOrderId();
    const now = store.now();
    const rec: OrderRecord = {
      id, player_id: str(body, "player_id"),
      product_id: nonEmpty(str(body, "product_id"), "product.demo"),
      amount: num(body, 0, "amount"), currency: nonEmpty(str(body, "currency"), "CNY"),
      status: nonEmpty(str(body, "status"), "created"),
      channel: nonEmpty(str(body, "channel"), "gm"),
      created_at: now, updated_at: now,
      attributes: body.attributes as Record<string, unknown> | undefined,
    };
    store.orders.set(id, rec);
    return resp({ status: "success", action: "order.create", order: rec });
  };
}

function orderGet(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const r = store.orders.get(str(body, "order_id", "id"));
    if (!r) return resp({ status: "not_found", message: "order not found" });
    return resp({ status: "success", action: "order.get", order: r });
  };
}

function orderUpdate(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const r = store.orders.get(str(body, "order_id", "id"));
    if (!r) return resp({ status: "not_found", message: "order not found" });
    const status = str(body, "status"); if (status) r.status = status;
    const channel = str(body, "channel"); if (channel) r.channel = channel;
    if ("amount" in body) r.amount = num(body, r.amount, "amount");
    if (body.attributes && typeof body.attributes === "object") r.attributes = body.attributes as Record<string, unknown>;
    r.updated_at = store.now();
    return resp({ status: "success", action: "order.update", order: r });
  };
}

function orderDelete(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const id = str(body, "order_id", "id");
    store.orders.delete(id);
    return resp({ status: "success", action: "order.delete", order_id: id });
  };
}

function orderList(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const pid = str(body, "player_id");
    const items = [...store.orders.values()]
      .filter(o => !pid || o.player_id === pid)
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
    const pid = str(body, "player_id");
    if (!pid) throw new Error("player_id is required");
    const p = store.players.get(pid);
    const entry: LeaderboardEntry = {
      player_id: pid, player_name: p?.name || pid,
      score: num(body, 0, "score"), rank: 0, updated_at: store.now(),
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
    const pid = str(body, "player_id");
    if (!pid) throw new Error("player_id is required");
    const inv = store.inventories.get(pid) || new Map();
    const items = [...inv.values()].sort((a, b) => a.template_id.localeCompare(b.template_id));
    return resp({ status: "success", action: "inventory.list", player_id: pid, items });
  };
}

function inventoryGrant(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const pid = str(body, "player_id");
    const tid = str(body, "template_id", "item_id");
    if (!pid || !tid) throw new Error("player_id and template_id are required");
    if (!store.inventories.has(pid)) store.inventories.set(pid, new Map());
    const inv = store.inventories.get(pid)!;
    let r = inv.get(tid);
    if (!r) {
      r = { id: `item_${tid}`, template_id: tid, name: nonEmpty(str(body, "name"), tid),
            quantity: 0, rarity: nonEmpty(str(body, "rarity"), "common"), updated_at: "" };
      inv.set(tid, r);
    }
    r.quantity += num(body, 1, "quantity");
    r.updated_at = store.now();
    return resp({ status: "success", action: "inventory.grant", player_id: pid, item: r });
  };
}

function inventoryConsume(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const pid = str(body, "player_id");
    const tid = str(body, "template_id", "item_id");
    const qty = num(body, 1, "quantity");
    if (!pid || !tid) throw new Error("player_id and template_id are required");
    const inv = store.inventories.get(pid);
    const r = inv?.get(tid);
    if (!r) return resp({ status: "not_found", message: "item not found" });
    if (r.quantity < qty) return resp({ status: "failed", message: "insufficient quantity", item: r });
    r.quantity -= qty;
    r.updated_at = store.now();
    return resp({ status: "success", action: "inventory.consume", player_id: pid, item: r });
  };
}

function mailSend(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const pid = str(body, "player_id");
    if (!pid) throw new Error("player_id is required");
    const now = store.now();
    const rec: MailRecord = {
      id: store.nextMailId(), player_id: pid,
      title: nonEmpty(str(body, "title"), "系统邮件"),
      content: nonEmpty(str(body, "content"), "请查收奖励"),
      status: "unread", reward: body.reward as Record<string, unknown> | undefined,
      sent_at: now, updated_at: now, expire_at: str(body, "expire_at") || undefined,
    };
    if (!store.mails.has(pid)) store.mails.set(pid, []);
    store.mails.get(pid)!.push(rec);
    return resp({ status: "success", action: "mail.send", mail: rec });
  };
}

function mailList(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const pid = str(body, "player_id");
    if (!pid) throw new Error("player_id is required");
    const items = store.mails.get(pid) || [];
    return resp({ status: "success", action: "mail.list", player_id: pid, items, total: items.length });
  };
}

function mailClaim(store: DemoStore): FunctionHandler {
  return async (_ctx, payload) => {
    const body = parsePayload(payload);
    const pid = str(body, "player_id");
    const mid = str(body, "mail_id", "id");
    if (!pid || !mid) throw new Error("player_id and mail_id are required");
    const list = store.mails.get(pid) || [];
    const m = list.find(x => x.id === mid);
    if (!m) return resp({ status: "not_found", message: "mail not found" });
    m.status = "claimed"; m.updated_at = store.now();
    return resp({ status: "success", action: "mail.claim", mail: m });
  };
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

  const fns: Array<[string, string, string, string, string, FunctionHandler]> = [
    ["player.create", "player", "medium", "player", "create", playerCreate(store)],
    ["player.get", "player", "low", "player", "read", playerGet(store)],
    ["player.update", "player", "medium", "player", "update", playerUpdate(store)],
    ["player.delete", "player", "high", "player", "delete", playerDelete(store)],
    ["player.list", "player", "low", "player", "read", playerList(store)],
    ["order.create", "commerce", "medium", "order", "create", orderCreate(store)],
    ["order.get", "commerce", "low", "order", "read", orderGet(store)],
    ["order.update", "commerce", "medium", "order", "update", orderUpdate(store)],
    ["order.delete", "commerce", "high", "order", "delete", orderDelete(store)],
    ["order.list", "commerce", "low", "order", "read", orderList(store)],
    ["leaderboard.list", "leaderboard", "low", "leaderboard", "read", leaderboardList(store)],
    ["leaderboard.upsert", "leaderboard", "medium", "leaderboard", "update", leaderboardUpsert(store)],
    ["leaderboard.reset", "leaderboard", "high", "leaderboard", "delete", leaderboardReset(store)],
    ["inventory.list", "inventory", "low", "inventory", "read", inventoryList(store)],
    ["inventory.grant", "inventory", "medium", "inventory", "create", inventoryGrant(store)],
    ["inventory.consume", "inventory", "medium", "inventory", "delete", inventoryConsume(store)],
    ["mail.send", "mail", "medium", "mail", "create", mailSend(store)],
    ["mail.list", "mail", "low", "mail", "read", mailList(store)],
    ["mail.claim", "mail", "medium", "mail", "update", mailClaim(store)],
  ];

  for (const [id, cat, risk, entity, op, handler] of fns) {
    const desc: FunctionDescriptor = {
      id, version: "1.0.0", category: cat, risk, entity, operation: op,
    };
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
