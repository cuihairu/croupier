package com.croupier.sdk.examples;

import io.github.cuihairu.croupier.sdk.*;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Instant;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.atomic.AtomicLong;

/**
 * Game Demo - 19 functions matching the Go SDK demo.
 *
 * Covers: player/order lifecycle actions, leaderboard, inventory, and mail.
 * Run: cd sdks/java && mvn exec:java -Dexec.mainClass=com.croupier.sdk.examples.GameDemo
 */
public class GameDemo {
    private static final Logger log = LoggerFactory.getLogger(GameDemo.class);

    // ==================== Data Models ====================

    static class PlayerRecord {
        public String id, name, status, server, createdAt, updatedAt, lastLoginAt;
        public int level, vip;
        public long gold;
        public Map<String, Object> profile;

        Map<String, Object> toMap() {
            Map<String, Object> m = new LinkedHashMap<>();
            m.put("id", id); m.put("name", name); m.put("level", level);
            m.put("vip", vip); m.put("gold", gold); m.put("status", status);
            m.put("server", server); m.put("createdAt", createdAt);
            m.put("updatedAt", updatedAt); m.put("last_login_at", lastLoginAt);
            if (profile != null) m.put("profile", profile);
            return m;
        }
    }

    static class OrderRecord {
        public String id, playerId, productId, currency, status, channel, createdAt, updatedAt;
        public long amount;
        public Map<String, Object> attributes;

        Map<String, Object> toMap() {
            Map<String, Object> m = new LinkedHashMap<>();
            m.put("id", id); m.put("playerId", playerId); m.put("productId", productId);
            m.put("amount", amount); m.put("currency", currency); m.put("status", status);
            m.put("channel", channel); m.put("createdAt", createdAt);
            m.put("updatedAt", updatedAt);
            if (attributes != null) m.put("attributes", attributes);
            return m;
        }
    }

    static class LeaderboardEntry {
        public String playerId, playerName, updatedAt;
        public long score;
        public int rank;

        Map<String, Object> toMap() {
            Map<String, Object> m = new LinkedHashMap<>();
            m.put("playerId", playerId); m.put("playerName", playerName);
            m.put("score", score); m.put("rank", rank); m.put("updatedAt", updatedAt);
            return m;
        }
    }

    static class ItemRecord {
        public String id, templateId, name, rarity, updatedAt;
        public long quantity;

        Map<String, Object> toMap() {
            Map<String, Object> m = new LinkedHashMap<>();
            m.put("id", id); m.put("templateId", templateId); m.put("name", name);
            m.put("quantity", quantity); m.put("rarity", rarity); m.put("updatedAt", updatedAt);
            return m;
        }
    }

    static class MailRecord {
        public String id, playerId, title, content, status, sentAt, updatedAt, expireAt;
        public Map<String, Object> reward;

        Map<String, Object> toMap() {
            Map<String, Object> m = new LinkedHashMap<>();
            m.put("id", id); m.put("playerId", playerId); m.put("title", title);
            m.put("content", content); m.put("status", status);
            if (reward != null) m.put("reward", reward);
            m.put("sentAt", sentAt); m.put("updatedAt", updatedAt);
            if (expireAt != null) m.put("expireAt", expireAt);
            return m;
        }
    }

    // ==================== In-Memory Store ====================

    static class DemoStore {
        final AtomicLong playerSeq = new AtomicLong(1002);
        final AtomicLong orderSeq = new AtomicLong(3002);
        final AtomicLong mailSeq = new AtomicLong(5002);
        final ConcurrentHashMap<String, PlayerRecord> players = new ConcurrentHashMap<>();
        final ConcurrentHashMap<String, OrderRecord> orders = new ConcurrentHashMap<>();
        final ConcurrentHashMap<String, LeaderboardEntry> leaderboard = new ConcurrentHashMap<>();
        final ConcurrentHashMap<String, ConcurrentHashMap<String, ItemRecord>> inventories = new ConcurrentHashMap<>();
        final ConcurrentHashMap<String, CopyOnWriteArrayList<MailRecord>> mails = new ConcurrentHashMap<>();

        DemoStore() {
            String now = Instant.now().toString();
            // Seed players
            PlayerRecord alice = new PlayerRecord();
            alice.id = "player_1001"; alice.name = "Alice"; alice.level = 35; alice.vip = 3;
            alice.gold = 128800; alice.status = "active"; alice.server = "s1";
            alice.createdAt = now; alice.updatedAt = now; alice.lastLoginAt = now;
            alice.profile = Map.of("guild", "星海旅团", "country", "CN", "platform", "ios");
            players.put(alice.id, alice);

            PlayerRecord bob = new PlayerRecord();
            bob.id = "player_1002"; bob.name = "Bob"; bob.level = 42; bob.vip = 5;
            bob.gold = 256000; bob.status = "active"; bob.server = "s2";
            bob.createdAt = now; bob.updatedAt = now; bob.lastLoginAt = now;
            bob.profile = Map.of("guild", "苍穹守卫", "country", "US", "platform", "android");
            players.put(bob.id, bob);

            // Seed orders
            OrderRecord o1 = new OrderRecord();
            o1.id = "order_3001"; o1.playerId = "player_1001"; o1.productId = "com.croupier.gems.648";
            o1.amount = 6480; o1.currency = "CNY"; o1.status = "paid"; o1.channel = "appstore";
            o1.createdAt = now; o1.updatedAt = now; o1.attributes = Map.of("region", "cn");
            orders.put(o1.id, o1);

            OrderRecord o2 = new OrderRecord();
            o2.id = "order_3002"; o2.playerId = "player_1002"; o2.productId = "battle.pass.s2";
            o2.amount = 68; o2.currency = "USD"; o2.status = "pending"; o2.channel = "googleplay";
            o2.createdAt = now; o2.updatedAt = now;
            orders.put(o2.id, o2);

            // Seed leaderboard
            LeaderboardEntry lb1 = new LeaderboardEntry();
            lb1.playerId = "player_1002"; lb1.playerName = "Bob"; lb1.score = 98500; lb1.rank = 1; lb1.updatedAt = now;
            leaderboard.put(lb1.playerId, lb1);
            LeaderboardEntry lb2 = new LeaderboardEntry();
            lb2.playerId = "player_1001"; lb2.playerName = "Alice"; lb2.score = 91200; lb2.rank = 2; lb2.updatedAt = now;
            leaderboard.put(lb2.playerId, lb2);

            // Seed inventory
            ConcurrentHashMap<String, ItemRecord> inv = new ConcurrentHashMap<>();
            ItemRecord gold = new ItemRecord();
            gold.id = "item_gold_coin"; gold.templateId = "gold_coin"; gold.name = "金币";
            gold.quantity = 128800; gold.rarity = "common"; gold.updatedAt = now;
            inv.put(gold.templateId, gold);
            ItemRecord ticket = new ItemRecord();
            ticket.id = "item_hero_ticket"; ticket.templateId = "hero_ticket"; ticket.name = "英雄招募券";
            ticket.quantity = 12; ticket.rarity = "rare"; ticket.updatedAt = now;
            inv.put(ticket.templateId, ticket);
            inventories.put("player_1001", inv);

            // Seed mail
            MailRecord mail = new MailRecord();
            mail.id = "mail_5001"; mail.playerId = "player_1001"; mail.title = "开服奖励";
            mail.content = "欢迎来到 Croupier Demo World"; mail.status = "unread";
            mail.reward = Map.of("gold", 10000, "item", "hero_ticket");
            mail.sentAt = now; mail.updatedAt = now;
            mails.put("player_1001", new CopyOnWriteArrayList<>(List.of(mail)));
        }
    }

    // ==================== JSON Helpers ====================

    @SuppressWarnings("unchecked")
    static Map<String, Object> parseJson(String s) {
        if (s == null || s.isBlank()) return new HashMap<>();
        try {
            // Simple JSON parse using built-in (no external dependency)
            return (Map<String, Object>) SimpleJson.parse(s);
        } catch (Exception e) {
            return new HashMap<>();
        }
    }

    static String toJson(Map<String, Object> m) {
        return SimpleJson.stringify(m);
    }

    static String str(Map<String, Object> m, String... keys) {
        for (String k : keys) {
            Object v = m.get(k);
            if (v instanceof String s && !s.isBlank()) return s.trim();
        }
        return "";
    }

    static long lng(Map<String, Object> m, long def, String... keys) {
        for (String k : keys) {
            Object v = m.get(k);
            if (v instanceof Number n) return n.longValue();
            if (v instanceof String s) try { return Long.parseLong(s.trim()); } catch (Exception ignored) {}
        }
        return def;
    }

    static int integer(Map<String, Object> m, int def, String... keys) {
        return (int) lng(m, def, keys);
    }

    @SuppressWarnings("unchecked")
    static Map<String, Object> mapVal(Map<String, Object> m, String key) {
        Object v = m.get(key);
        return v instanceof Map ? (Map<String, Object>) v : null;
    }

    // ==================== Function Handlers ====================

    static FunctionHandler playerCreate(DemoStore store) {
        return (ctx, payload) -> {
            Map<String, Object> body = parseJson(payload);
            String id = str(body, "id", "playerId");
            if (id.isEmpty()) id = "player_" + store.playerSeq.incrementAndGet();
            String now = Instant.now().toString();
            PlayerRecord r = new PlayerRecord();
            r.id = id; r.name = nonEmpty(str(body, "name"), "Player-" + id);
            r.level = integer(body, 1, "level"); r.vip = integer(body, 0, "vip");
            r.gold = lng(body, 0, "gold"); r.status = nonEmpty(str(body, "status"), "active");
            r.server = nonEmpty(str(body, "server"), "s1");
            r.createdAt = now; r.updatedAt = now; r.lastLoginAt = now;
            r.profile = mapVal(body, "profile");
            store.players.put(id, r);
            return toJson(Map.of("status", "success", "action", "player.create", "player", r.toMap()));
        };
    }

    static FunctionHandler playerGet(DemoStore store) {
        return (ctx, payload) -> {
            Map<String, Object> body = parseJson(payload);
            String id = str(body, "playerId", "id");
            PlayerRecord r = store.players.get(id);
            if (r == null) return toJson(Map.of("status", "not_found", "message", "player not found"));
            return toJson(Map.of("status", "success", "action", "player.get", "player", r.toMap()));
        };
    }

    static FunctionHandler playerUpdate(DemoStore store) {
        return (ctx, payload) -> {
            Map<String, Object> body = parseJson(payload);
            String id = str(body, "playerId", "id");
            PlayerRecord r = store.players.get(id);
            if (r == null) return toJson(Map.of("status", "not_found", "message", "player not found"));
            String name = str(body, "name"); if (!name.isEmpty()) r.name = name;
            if (body.containsKey("level")) r.level = integer(body, r.level, "level");
            if (body.containsKey("vip")) r.vip = integer(body, r.vip, "vip");
            if (body.containsKey("gold")) r.gold = lng(body, r.gold, "gold");
            String status = str(body, "status"); if (!status.isEmpty()) r.status = status;
            String server = str(body, "server"); if (!server.isEmpty()) r.server = server;
            Map<String, Object> profile = mapVal(body, "profile"); if (profile != null) r.profile = profile;
            r.updatedAt = Instant.now().toString();
            return toJson(Map.of("status", "success", "action", "player.update", "player", r.toMap()));
        };
    }

    static FunctionHandler playerDelete(DemoStore store) {
        return (ctx, payload) -> {
            Map<String, Object> body = parseJson(payload);
            String id = str(body, "playerId", "id");
            store.players.remove(id);
            store.inventories.remove(id);
            store.mails.remove(id);
            store.leaderboard.remove(id);
            return toJson(Map.of("status", "success", "action", "player.delete", "playerId", id));
        };
    }

    static FunctionHandler playerList(DemoStore store) {
        return (ctx, payload) -> {
            List<Map<String, Object>> items = store.players.values().stream()
                .sorted(Comparator.comparing(p -> p.id))
                .map(PlayerRecord::toMap).toList();
            return toJson(Map.of("status", "success", "action", "player.list", "items", items, "total", items.size()));
        };
    }

    static FunctionHandler orderCreate(DemoStore store) {
        return (ctx, payload) -> {
            Map<String, Object> body = parseJson(payload);
            String id = str(body, "order_id", "id");
            if (id.isEmpty()) id = "order_" + store.orderSeq.incrementAndGet();
            String now = Instant.now().toString();
            OrderRecord r = new OrderRecord();
            r.id = id; r.playerId = str(body, "playerId");
            r.productId = nonEmpty(str(body, "productId"), "product.demo");
            r.amount = lng(body, 0, "amount"); r.currency = nonEmpty(str(body, "currency"), "CNY");
            r.status = nonEmpty(str(body, "status"), "created");
            r.channel = nonEmpty(str(body, "channel"), "gm");
            r.createdAt = now; r.updatedAt = now; r.attributes = mapVal(body, "attributes");
            store.orders.put(id, r);
            return toJson(Map.of("status", "success", "action", "order.create", "order", r.toMap()));
        };
    }

    static FunctionHandler orderGet(DemoStore store) {
        return (ctx, payload) -> {
            Map<String, Object> body = parseJson(payload);
            String id = str(body, "order_id", "id");
            OrderRecord r = store.orders.get(id);
            if (r == null) return toJson(Map.of("status", "not_found", "message", "order not found"));
            return toJson(Map.of("status", "success", "action", "order.get", "order", r.toMap()));
        };
    }

    static FunctionHandler orderUpdate(DemoStore store) {
        return (ctx, payload) -> {
            Map<String, Object> body = parseJson(payload);
            String id = str(body, "order_id", "id");
            OrderRecord r = store.orders.get(id);
            if (r == null) return toJson(Map.of("status", "not_found", "message", "order not found"));
            String status = str(body, "status"); if (!status.isEmpty()) r.status = status;
            String channel = str(body, "channel"); if (!channel.isEmpty()) r.channel = channel;
            if (body.containsKey("amount")) r.amount = lng(body, r.amount, "amount");
            Map<String, Object> attrs = mapVal(body, "attributes"); if (attrs != null) r.attributes = attrs;
            r.updatedAt = Instant.now().toString();
            return toJson(Map.of("status", "success", "action", "order.update", "order", r.toMap()));
        };
    }

    static FunctionHandler orderDelete(DemoStore store) {
        return (ctx, payload) -> {
            Map<String, Object> body = parseJson(payload);
            String id = str(body, "order_id", "id");
            store.orders.remove(id);
            return toJson(Map.of("status", "success", "action", "order.delete", "order_id", id));
        };
    }

    static FunctionHandler orderList(DemoStore store) {
        return (ctx, payload) -> {
            Map<String, Object> body = parseJson(payload);
            String playerId = str(body, "playerId");
            List<Map<String, Object>> items = store.orders.values().stream()
                .filter(o -> playerId.isEmpty() || o.playerId.equals(playerId))
                .sorted(Comparator.comparing(o -> o.id))
                .map(OrderRecord::toMap).toList();
            return toJson(Map.of("status", "success", "action", "order.list", "items", items, "total", items.size()));
        };
    }

    static FunctionHandler leaderboardList(DemoStore store) {
        return (ctx, payload) -> {
            List<Map<String, Object>> items = store.leaderboard.values().stream()
                .sorted((a, b) -> Long.compare(b.score, a.score))
                .map(LeaderboardEntry::toMap).toList();
            // Re-rank
            for (int i = 0; i < items.size(); i++) items.get(i).put("rank", i + 1);
            return toJson(Map.of("status", "success", "action", "leaderboard.list", "items", items, "total", items.size()));
        };
    }

    static FunctionHandler leaderboardUpsert(DemoStore store) {
        return (ctx, payload) -> {
            Map<String, Object> body = parseJson(payload);
            String playerId = str(body, "playerId");
            if (playerId.isEmpty()) throw new IllegalArgumentException("player_id is required");
            String playerName = playerId;
            PlayerRecord p = store.players.get(playerId);
            if (p != null && p.name != null) playerName = p.name;
            LeaderboardEntry e = new LeaderboardEntry();
            e.playerId = playerId; e.playerName = playerName;
            e.score = lng(body, 0, "score"); e.updatedAt = Instant.now().toString();
            store.leaderboard.put(playerId, e);
            return toJson(Map.of("status", "success", "action", "leaderboard.upsert", "entry", e.toMap()));
        };
    }

    static FunctionHandler leaderboardReset(DemoStore store) {
        return (ctx, payload) -> {
            store.leaderboard.clear();
            return toJson(Map.of("status", "success", "action", "leaderboard.reset"));
        };
    }

    static FunctionHandler inventoryList(DemoStore store) {
        return (ctx, payload) -> {
            Map<String, Object> body = parseJson(payload);
            String playerId = str(body, "playerId");
            if (playerId.isEmpty()) throw new IllegalArgumentException("player_id is required");
            ConcurrentHashMap<String, ItemRecord> inv = store.inventories.getOrDefault(playerId, new ConcurrentHashMap<>());
            List<Map<String, Object>> items = inv.values().stream()
                .sorted(Comparator.comparing(i -> i.templateId))
                .map(ItemRecord::toMap).toList();
            return toJson(Map.of("status", "success", "action", "inventory.list", "playerId", playerId, "items", items));
        };
    }

    static FunctionHandler inventoryGrant(DemoStore store) {
        return (ctx, payload) -> {
            Map<String, Object> body = parseJson(payload);
            String playerId = str(body, "playerId");
            String templateId = str(body, "templateId", "item_id");
            if (playerId.isEmpty() || templateId.isEmpty()) throw new IllegalArgumentException("player_id and template_id are required");
            store.inventories.computeIfAbsent(playerId, k -> new ConcurrentHashMap<>());
            ConcurrentHashMap<String, ItemRecord> inv = store.inventories.get(playerId);
            ItemRecord r = inv.get(templateId);
            if (r == null) {
                r = new ItemRecord();
                r.id = "item_" + templateId; r.templateId = templateId;
                r.name = nonEmpty(str(body, "name"), templateId);
                r.rarity = nonEmpty(str(body, "rarity"), "common");
                inv.put(templateId, r);
            }
            r.quantity += lng(body, 1, "quantity");
            r.updatedAt = Instant.now().toString();
            return toJson(Map.of("status", "success", "action", "inventory.grant", "playerId", playerId, "item", r.toMap()));
        };
    }

    static FunctionHandler inventoryConsume(DemoStore store) {
        return (ctx, payload) -> {
            Map<String, Object> body = parseJson(payload);
            String playerId = str(body, "playerId");
            String templateId = str(body, "templateId", "item_id");
            long quantity = lng(body, 1, "quantity");
            if (playerId.isEmpty() || templateId.isEmpty()) throw new IllegalArgumentException("player_id and template_id are required");
            ConcurrentHashMap<String, ItemRecord> inv = store.inventories.get(playerId);
            if (inv == null || !inv.containsKey(templateId))
                return toJson(Map.of("status", "not_found", "message", "item not found"));
            ItemRecord r = inv.get(templateId);
            if (r.quantity < quantity)
                return toJson(Map.of("status", "failed", "message", "insufficient quantity", "item", r.toMap()));
            r.quantity -= quantity;
            r.updatedAt = Instant.now().toString();
            return toJson(Map.of("status", "success", "action", "inventory.consume", "playerId", playerId, "item", r.toMap()));
        };
    }

    static FunctionHandler mailSend(DemoStore store) {
        return (ctx, payload) -> {
            Map<String, Object> body = parseJson(payload);
            String playerId = str(body, "playerId");
            if (playerId.isEmpty()) throw new IllegalArgumentException("player_id is required");
            String now = Instant.now().toString();
            MailRecord r = new MailRecord();
            r.id = "mail_" + store.mailSeq.incrementAndGet(); r.playerId = playerId;
            r.title = nonEmpty(str(body, "title"), "系统邮件");
            r.content = nonEmpty(str(body, "content"), "请查收奖励");
            r.status = "unread"; r.reward = mapVal(body, "reward");
            r.sentAt = now; r.updatedAt = now; r.expireAt = str(body, "expireAt");
            store.mails.computeIfAbsent(playerId, k -> new CopyOnWriteArrayList<>()).add(r);
            return toJson(Map.of("status", "success", "action", "mail.send", "mail", r.toMap()));
        };
    }

    static FunctionHandler mailList(DemoStore store) {
        return (ctx, payload) -> {
            Map<String, Object> body = parseJson(payload);
            String playerId = str(body, "playerId");
            if (playerId.isEmpty()) throw new IllegalArgumentException("player_id is required");
            List<Map<String, Object>> items = store.mails.getOrDefault(playerId, new CopyOnWriteArrayList<>())
                .stream().map(MailRecord::toMap).toList();
            return toJson(Map.of("status", "success", "action", "mail.list", "playerId", playerId, "items", items, "total", items.size()));
        };
    }

    static FunctionHandler mailClaim(DemoStore store) {
        return (ctx, payload) -> {
            Map<String, Object> body = parseJson(payload);
            String playerId = str(body, "playerId");
            String mailId = str(body, "mail_id", "id");
            if (playerId.isEmpty() || mailId.isEmpty()) throw new IllegalArgumentException("player_id and mail_id are required");
            List<MailRecord> list = store.mails.get(playerId);
            if (list != null) {
                for (MailRecord m : list) {
                    if (m.id.equals(mailId)) {
                        m.status = "claimed"; m.updatedAt = Instant.now().toString();
                        return toJson(Map.of("status", "success", "action", "mail.claim", "mail", m.toMap()));
                    }
                }
            }
            return toJson(Map.of("status", "not_found", "message", "mail not found"));
        };
    }

    // ==================== Registration ====================

    static void registerAll(CroupierClient client, DemoStore store) throws CroupierException {
        record Fn(String id, String risk, String resource, String op, FunctionHandler handler) {}
        List<Fn> fns = List.of(
            new Fn("player.create", "warning", "player", "create", playerCreate(store)),
            new Fn("player.get", "safe", "player", "get", playerGet(store)),
            new Fn("player.update", "warning", "player", "update", playerUpdate(store)),
            new Fn("player.delete", "danger", "player", "delete", playerDelete(store)),
            new Fn("player.list", "safe", "player", "list", playerList(store)),
            new Fn("order.create", "warning", "order", "create", orderCreate(store)),
            new Fn("order.get", "safe", "order", "get", orderGet(store)),
            new Fn("order.update", "warning", "order", "update", orderUpdate(store)),
            new Fn("order.delete", "danger", "order", "delete", orderDelete(store)),
            new Fn("order.list", "safe", "order", "list", orderList(store)),
            new Fn("leaderboard.list", "safe", "leaderboard", "list", leaderboardList(store)),
            new Fn("leaderboard.upsert", "warning", "leaderboard", "upsert", leaderboardUpsert(store)),
            new Fn("leaderboard.reset", "danger", "leaderboard", "reset", leaderboardReset(store)),
            new Fn("inventory.list", "safe", "inventory", "list", inventoryList(store)),
            new Fn("inventory.grant", "warning", "inventory", "grant", inventoryGrant(store)),
            new Fn("inventory.consume", "warning", "inventory", "consume", inventoryConsume(store)),
            new Fn("mail.send", "warning", "mail", "send", mailSend(store)),
            new Fn("mail.list", "safe", "mail", "list", mailList(store)),
            new Fn("mail.claim", "warning", "mail", "claim", mailClaim(store))
        );
        for (Fn f : fns) {
            FunctionDescriptor desc = new FunctionDescriptor(f.id, "1.0.0");
            desc.setRisk(f.risk); desc.setResource(f.resource);
            desc.setOperation(f.op); desc.setEnabled(true);
            enrichDescriptor(desc);
            client.registerFunction(desc, f.handler);
            log.info("registered: {}", f.id);
        }
    }

    private static void enrichDescriptor(FunctionDescriptor desc) {
        desc.setTags(List.of(desc.getResource(), desc.getOperation()));
        desc.setSummary(desc.getResource() + " " + desc.getOperation());
        desc.setDescription(String.format(
            "Demo function %s for %s %s action.",
            desc.getId(), desc.getResource(), desc.getOperation()
        ));
        desc.setOperationId(desc.getId());
        String[] schemas = SCHEMAS.get(desc.getId());
        if (schemas != null) {
            desc.setInputSchema(schemas[0]);
            desc.setOutputSchema(schemas[1]);
            return;
        }
        desc.setInputSchema("{\"type\":\"object\",\"properties\":{}}");
        desc.setOutputSchema("{\"type\":\"object\",\"properties\":{\"status\":{\"type\":\"string\"},\"action\":{\"type\":\"string\"}}}");
    }

    // Schemas describe the handlers' real wire contract with camelCase JSON
    // keys. snake_case is only allowed inside databases, never on the wire.
    private static final String OBJ = "{\"type\":\"object\"}";
    private static final String STR = "{\"type\":\"string\"}";
    private static final String INT = "{\"type\":\"integer\"}";
    private static final String PLAYER_FIELDS =
        "{\"id\":" + STR + ",\"name\":" + STR + ",\"level\":" + INT + ",\"vip\":" + INT
            + ",\"gold\":" + INT + ",\"status\":" + STR + ",\"server\":" + STR + ",\"profile\":" + OBJ + "}";

    private static final Map<String, String[]> SCHEMAS = Map.ofEntries(
        Map.entry("player.create", new String[]{
            "{\"type\":\"object\",\"properties\":" + PLAYER_FIELDS + "}",
            "{\"type\":\"object\",\"properties\":{\"player\":" + OBJ + "}}"}),
        Map.entry("player.get", new String[]{
            "{\"type\":\"object\",\"properties\":{\"id\":" + STR + "},\"required\":[\"id\"]}",
            "{\"type\":\"object\",\"properties\":{\"player\":" + OBJ + "}}"}),
        Map.entry("player.update", new String[]{
            "{\"type\":\"object\",\"properties\":" + PLAYER_FIELDS + ",\"required\":[\"id\"]}",
            "{\"type\":\"object\",\"properties\":{\"player\":" + OBJ + "}}"}),
        Map.entry("player.delete", new String[]{
            "{\"type\":\"object\",\"properties\":{\"id\":" + STR + "},\"required\":[\"id\"]}",
            "{\"type\":\"object\",\"properties\":{\"playerId\":" + STR + "}}"}),
        Map.entry("player.list", new String[]{
            "{\"type\":\"object\",\"properties\":{\"page\":" + INT + ",\"pageSize\":" + INT + "}}",
            "{\"type\":\"object\",\"properties\":{\"items\":{\"type\":\"array\",\"items\":" + OBJ + "},\"total\":" + INT + "}}"}),
        Map.entry("order.create", new String[]{
            "{\"type\":\"object\",\"properties\":{\"id\":" + STR + ",\"playerId\":" + STR + ",\"productId\":" + STR + ",\"amount\":" + INT + ",\"currency\":" + STR + ",\"status\":" + STR + ",\"channel\":" + STR + ",\"attributes\":" + OBJ + "}}",
            "{\"type\":\"object\",\"properties\":{\"order\":" + OBJ + "}}"}),
        Map.entry("order.get", new String[]{
            "{\"type\":\"object\",\"properties\":{\"id\":" + STR + "},\"required\":[\"id\"]}",
            "{\"type\":\"object\",\"properties\":{\"order\":" + OBJ + "}}"}),
        Map.entry("order.update", new String[]{
            "{\"type\":\"object\",\"properties\":{\"id\":" + STR + ",\"status\":" + STR + ",\"channel\":" + STR + ",\"amount\":" + INT + ",\"attributes\":" + OBJ + "},\"required\":[\"id\"]}",
            "{\"type\":\"object\",\"properties\":{\"order\":" + OBJ + "}}"}),
        Map.entry("order.delete", new String[]{
            "{\"type\":\"object\",\"properties\":{\"id\":" + STR + "},\"required\":[\"id\"]}",
            "{\"type\":\"object\",\"properties\":{\"orderId\":" + STR + "}}"}),
        Map.entry("order.list", new String[]{
            "{\"type\":\"object\",\"properties\":{\"playerId\":" + STR + ",\"page\":" + INT + ",\"pageSize\":" + INT + "}}",
            "{\"type\":\"object\",\"properties\":{\"items\":{\"type\":\"array\",\"items\":" + OBJ + "},\"total\":" + INT + "}}"}),
        Map.entry("leaderboard.list", new String[]{
            "{\"type\":\"object\",\"properties\":{\"page\":" + INT + ",\"pageSize\":" + INT + "}}",
            "{\"type\":\"object\",\"properties\":{\"items\":{\"type\":\"array\",\"items\":" + OBJ + "},\"total\":" + INT + "}}"}),
        Map.entry("leaderboard.upsert", new String[]{
            "{\"type\":\"object\",\"properties\":{\"playerId\":" + STR + ",\"score\":" + INT + "},\"required\":[\"playerId\"]}",
            "{\"type\":\"object\",\"properties\":{\"entry\":" + OBJ + "}}"}),
        Map.entry("leaderboard.reset", new String[]{"{\"type\":\"object\",\"properties\":{}}", "{\"type\":\"object\",\"properties\":{}}"}),
        Map.entry("inventory.list", new String[]{
            "{\"type\":\"object\",\"properties\":{\"playerId\":" + STR + "},\"required\":[\"playerId\"]}",
            "{\"type\":\"object\",\"properties\":{\"items\":{\"type\":\"array\",\"items\":" + OBJ + "}}}"}),
        Map.entry("inventory.grant", new String[]{
            "{\"type\":\"object\",\"properties\":{\"playerId\":" + STR + ",\"templateId\":" + STR + ",\"quantity\":" + INT + "},\"required\":[\"playerId\",\"templateId\"]}",
            "{\"type\":\"object\",\"properties\":{\"item\":" + OBJ + "}}"}),
        Map.entry("inventory.consume", new String[]{
            "{\"type\":\"object\",\"properties\":{\"playerId\":" + STR + ",\"templateId\":" + STR + ",\"quantity\":" + INT + "},\"required\":[\"playerId\",\"templateId\"]}",
            "{\"type\":\"object\",\"properties\":{\"item\":" + OBJ + "}}"}),
        Map.entry("mail.send", new String[]{
            "{\"type\":\"object\",\"properties\":{\"playerId\":" + STR + ",\"title\":" + STR + ",\"content\":" + STR + ",\"reward\":" + OBJ + ",\"expireAt\":" + STR + "},\"required\":[\"playerId\"]}",
            "{\"type\":\"object\",\"properties\":{\"mail\":" + OBJ + "}}"}),
        Map.entry("mail.list", new String[]{
            "{\"type\":\"object\",\"properties\":{\"playerId\":" + STR + "},\"required\":[\"playerId\"]}",
            "{\"type\":\"object\",\"properties\":{\"items\":{\"type\":\"array\",\"items\":" + OBJ + "},\"total\":" + INT + "}}"}),
        Map.entry("mail.claim", new String[]{
            "{\"type\":\"object\",\"properties\":{\"playerId\":" + STR + ",\"mailId\":" + STR + "},\"required\":[\"playerId\",\"mailId\"]}",
            "{\"type\":\"object\",\"properties\":{\"mail\":" + OBJ + "}}"})
    );

    // ==================== Main ====================

    public static void main(String[] args) throws Exception {
        String agentAddr = env("CROUPIER_AGENT_ADDR", "127.0.0.1:19091");
        String gameId = env("CROUPIER_GAME_ID", "demo-game");
        String serviceId = env("CROUPIER_SERVICE_ID", "game-demo-service");
        String envName = env("CROUPIER_ENV", "development");

        ClientConfig config = new ClientConfig(gameId, serviceId);
        config.setAgentAddr(agentAddr);
        config.setEnv(envName);
        config.setServiceVersion("1.0.0");
        config.setInsecure(true);

        log.info("starting game demo: agent={} game={} env={} service={}", agentAddr, gameId, envName, serviceId);

        CroupierClient client = CroupierSDK.createClient(config);
        DemoStore store = new DemoStore();
        registerAll(client, store);

        CountDownLatch latch = new CountDownLatch(1);
        Runtime.getRuntime().addShutdownHook(new Thread(() -> {
            log.info("shutting down...");
            client.stop();
            client.close();
            latch.countDown();
        }));

        client.serveAsync().thenRun(() -> log.info("service started"))
            .exceptionally(t -> { log.error("serve failed", t); latch.countDown(); return null; });

        latch.await();
    }

    static String env(String key, String def) {
        String v = System.getenv(key);
        return (v != null && !v.isBlank()) ? v : def;
    }

    static String nonEmpty(String... vals) {
        for (String v : vals) if (v != null && !v.isBlank()) return v.trim();
        return "";
    }

    // ==================== Minimal JSON (no external deps) ====================

    static class SimpleJson {
        static Object parse(String s) {
            s = s.strip();
            if (s.startsWith("{")) return parseObject(s);
            if (s.startsWith("[")) return parseArray(s);
            return parseValue(s);
        }

        static Map<String, Object> parseObject(String s) {
            Map<String, Object> m = new LinkedHashMap<>();
            s = s.substring(1, s.length() - 1).strip();
            if (s.isEmpty()) return m;
            int i = 0, depth = 0; boolean inStr = false; char prev = 0;
            StringBuilder key = null; List<String> keys = new ArrayList<>(); List<String> vals = new ArrayList<>();
            StringBuilder cur = new StringBuilder();
            for (int ci = 0; ci < s.length(); ci++) {
                char c = s.charAt(ci);
                if (inStr) { cur.append(c); if (c == '"' && prev != '\\') inStr = false; prev = c; continue; }
                if (c == '"') { inStr = true; cur.append(c); prev = c; continue; }
                if (c == '{' || c == '[') depth++;
                if (c == '}' || c == ']') depth--;
                if (c == ':' && depth == 0) { key = new StringBuilder(cur); cur.setLength(0); continue; }
                if (c == ',' && depth == 0) { keys.add(key.toString()); vals.add(cur.toString()); cur.setLength(0); continue; }
                cur.append(c); prev = c;
            }
            if (cur.length() > 0 && key != null) { keys.add(key.toString()); vals.add(cur.toString()); }
            for (int j = 0; j < keys.size(); j++) {
                String k = stripQuotes(keys.get(j).strip());
                m.put(k, parse(vals.get(j).strip()));
            }
            return m;
        }

        static List<Object> parseArray(String s) {
            List<Object> l = new ArrayList<>();
            s = s.substring(1, s.length() - 1).strip();
            if (s.isEmpty()) return l;
            int depth = 0; boolean inStr = false; char prev = 0;
            StringBuilder cur = new StringBuilder();
            for (int ci = 0; ci < s.length(); ci++) {
                char c = s.charAt(ci);
                if (inStr) { cur.append(c); if (c == '"' && prev != '\\') inStr = false; prev = c; continue; }
                if (c == '"') { inStr = true; cur.append(c); prev = c; continue; }
                if (c == '{' || c == '[') depth++;
                if (c == '}' || c == ']') depth--;
                if (c == ',' && depth == 0) { l.add(parse(cur.toString().strip())); cur.setLength(0); continue; }
                cur.append(c); prev = c;
            }
            if (cur.length() > 0) l.add(parse(cur.toString().strip()));
            return l;
        }

        static Object parseValue(String s) {
            if (s.equals("null")) return null;
            if (s.equals("true")) return true;
            if (s.equals("false")) return false;
            if (s.startsWith("\"")) return stripQuotes(s);
            try { return Long.parseLong(s); } catch (Exception e1) {
                try { return Double.parseDouble(s); } catch (Exception e2) { return s; }
            }
        }

        static String stripQuotes(String s) {
            if (s.startsWith("\"") && s.endsWith("\"") && s.length() >= 2)
                return s.substring(1, s.length() - 1).replace("\\\"", "\"").replace("\\n", "\n");
            return s;
        }

        static String stringify(Object obj) {
            if (obj == null) return "null";
            if (obj instanceof String s) return "\"" + s.replace("\"", "\\\"").replace("\n", "\\n") + "\"";
            if (obj instanceof Boolean || obj instanceof Number) return obj.toString();
            if (obj instanceof Map<?,?> m) {
                StringBuilder sb = new StringBuilder("{");
                boolean first = true;
                for (var e : m.entrySet()) {
                    if (!first) sb.append(","); first = false;
                    sb.append("\"").append(e.getKey()).append("\":").append(stringify(e.getValue()));
                }
                return sb.append("}").toString();
            }
            if (obj instanceof Collection<?> c) {
                StringBuilder sb = new StringBuilder("[");
                boolean first = true;
                for (Object item : c) { if (!first) sb.append(","); first = false; sb.append(stringify(item)); }
                return sb.append("]").toString();
            }
            if (obj instanceof Map.Entry<?,?> e) return "{\"" + e.getKey() + "\":" + stringify(e.getValue()) + "}";
            // Fallback: try toMap pattern
            try {
                var toMap = obj.getClass().getMethod("toMap");
                return stringify(toMap.invoke(obj));
            } catch (Exception ignored) {}
            return "\"" + obj.toString().replace("\"", "\\\"") + "\"";
        }
    }
}
