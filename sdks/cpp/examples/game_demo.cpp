/**
 * Game Demo - 18 functions matching the Go SDK demo contract.
 *
 * Covers: player/order lifecycle actions, leaderboard, inventory, and mail.
 * Every descriptor declares the v2 execution contract (capability/execution);
 * high-risk operations demonstrate approval_required/approval_policy_key, and
 * mail.batch_send demonstrates task execution for batch fan-out.
 * Build: cd sdks/cpp && cmake --build build --target croupier-game-demo
 * Run:   ./build/bin/croupier-game-demo
 */

#include "croupier/sdk/croupier_client.h"

#include <algorithm>
#include <atomic>
#include <chrono>
#include <csignal>
#include <iostream>
#include <map>
#include <mutex>
#include <sstream>
#include <string>
#include <utility>
#include <thread>
#include <vector>

using namespace croupier::sdk;

static std::atomic<bool> g_shutdown(false);

void signalHandler(int) { g_shutdown = true; }

// ==================== Helpers ====================

static std::string now_str() {
    auto now = std::chrono::system_clock::now();
    auto t = std::chrono::system_clock::to_time_t(now);
    char buf[64];
    std::strftime(buf, sizeof(buf), "%Y-%m-%dT%H:%M:%SZ", std::gmtime(&t));
    return buf;
}

static std::string json_str(const std::string& key, const std::string& val) {
    return "\"" + key + "\":\"" + val + "\"";
}
static std::string json_int(const std::string& key, long long val) {
    return "\"" + key + "\":" + std::to_string(val);
}

// Schemas describe the handlers' real wire contract with camelCase JSON
// keys. snake_case is only allowed inside databases, never on the wire.
static const char* SCHEMA_OBJ = "{\"type\":\"object\"}";
static const char* SCHEMA_STR = "{\"type\":\"string\"}";
static const char* SCHEMA_INT = "{\"type\":\"integer\"}";

static std::string player_fields_schema(bool with_required) {
    std::string required = with_required ? ",\"required\":[\"id\"]" : "";
    return "{\"type\":\"object\",\"properties\":{\"id\":" + std::string(SCHEMA_STR) +
           ",\"name\":" + SCHEMA_STR + ",\"level\":" + SCHEMA_INT + ",\"vip\":" + SCHEMA_INT +
           ",\"gold\":" + SCHEMA_INT + ",\"status\":" + SCHEMA_STR + ",\"server\":" + SCHEMA_STR +
           ",\"profile\":" + SCHEMA_OBJ + "}" + required + "}";
}

static std::pair<std::string, std::string> demo_schema_for(const std::string& id) {
    const std::string player_out = "{\"type\":\"object\",\"properties\":{\"player\":" + std::string(SCHEMA_OBJ) + "}}";
    const std::string list_out = "{\"type\":\"object\",\"properties\":{\"items\":{\"type\":\"array\",\"items\":" + std::string(SCHEMA_OBJ) + "},\"total\":" + SCHEMA_INT + "}}";
    const std::string pagination_in = "{\"type\":\"object\",\"properties\":{\"page\":" + std::string(SCHEMA_INT) + ",\"pageSize\":" + SCHEMA_INT + "}}";
    const std::string id_required_in = "{\"type\":\"object\",\"properties\":{\"id\":" + std::string(SCHEMA_STR) + "},\"required\":[\"id\"]}";
    if (id == "player.create") return {player_fields_schema(false), player_out};
    if (id == "player.get") return {id_required_in, player_out};
    if (id == "player.update") return {player_fields_schema(true), player_out};
    if (id == "player.delete") return {id_required_in, "{\"type\":\"object\",\"properties\":{\"playerId\":" + std::string(SCHEMA_STR) + "}}"};
    if (id == "player.list") return {pagination_in, list_out};
    // 与 Go demo 契约逐一对齐。此前未列出的函数落到 action fallback
    //（{status,action}），六语言 demo 共享同一契约槽位，fallback 注册
    // 会覆盖 Go 写入的正确 schema，导致页面发布校验
    // "/items,/total not found" 失败（DB 实证全量污染）。
    if (id == "order.list" || id == "leaderboard.list" ||
        id == "inventory.list" || id == "mail.list") return {pagination_in, list_out};
    if (id == "mail.batch_send") return {"{\"type\":\"object\",\"properties\":{\"mailIds\":{\"type\":\"array\",\"items\":{\"type\":\"string\"}},\"title\":{\"type\":\"string\"},\"content\":{\"type\":\"string\"},\"reward\":" + std::string(SCHEMA_OBJ) + "}}",
                                          "{\"type\":\"object\",\"properties\":{\"sent\":{\"type\":\"integer\"},\"failed\":{\"type\":\"integer\"}}}"};
    if (id == "leaderboard.reset") return {"{\"type\":\"object\",\"properties\":{}}",
                                           "{\"type\":\"object\",\"properties\":{\"reset\":{\"type\":\"boolean\"}}}"};
    if (id == "order.create") return {"{\"type\":\"object\",\"properties\":{\"id\":" + std::string(SCHEMA_STR) + ",\"playerId\":" + std::string(SCHEMA_STR) + ",\"productId\":" + std::string(SCHEMA_STR) + ",\"amount\":" + SCHEMA_INT + ",\"currency\":" + std::string(SCHEMA_STR) + ",\"status\":" + std::string(SCHEMA_STR) + ",\"channel\":" + std::string(SCHEMA_STR) + ",\"attributes\":" + std::string(SCHEMA_OBJ) + "}}",
                                      "{\"type\":\"object\",\"properties\":{\"order\":" + std::string(SCHEMA_OBJ) + "}}"};
    if (id == "order.get") return {"{\"type\":\"object\",\"properties\":{\"id\":" + std::string(SCHEMA_STR) + "},\"required\":[\"id\"]}",
                                   "{\"type\":\"object\",\"properties\":{\"order\":" + std::string(SCHEMA_OBJ) + "}}"};
    if (id == "order.update") return {"{\"type\":\"object\",\"properties\":{\"id\":" + std::string(SCHEMA_STR) + ",\"status\":" + std::string(SCHEMA_STR) + ",\"channel\":" + std::string(SCHEMA_STR) + ",\"amount\":" + SCHEMA_INT + "},\"required\":[\"id\"]}",
                                      "{\"type\":\"object\",\"properties\":{\"order\":" + std::string(SCHEMA_OBJ) + "}}"};
    if (id == "order.delete") return {"{\"type\":\"object\",\"properties\":{\"id\":" + std::string(SCHEMA_STR) + "},\"required\":[\"id\"]}",
                                      "{\"type\":\"object\",\"properties\":{\"deleted\":{\"type\":\"boolean\"}}}"};
    if (id == "leaderboard.upsert") return {"{\"type\":\"object\",\"properties\":{\"playerId\":" + std::string(SCHEMA_STR) + ",\"score\":" + SCHEMA_INT + "},\"required\":[\"playerId\"]}",
                                            "{\"type\":\"object\",\"properties\":{\"entry\":" + std::string(SCHEMA_OBJ) + "}}"};
    if (id == "inventory.grant" || id == "inventory.consume") return {"{\"type\":\"object\",\"properties\":{\"playerId\":" + std::string(SCHEMA_STR) + ",\"templateId\":" + std::string(SCHEMA_STR) + ",\"quantity\":" + SCHEMA_INT + "},\"required\":[\"playerId\",\"templateId\"]}",
                                                                       "{\"type\":\"object\",\"properties\":{\"item\":" + std::string(SCHEMA_OBJ) + "}}"};
    if (id == "mail.send" || id == "mail.claim") return {"{\"type\":\"object\",\"properties\":{\"playerId\":" + std::string(SCHEMA_STR) + ",\"mailId\":" + std::string(SCHEMA_STR) + "},\"required\":[\"playerId\"]}",
                                                          "{\"type\":\"object\",\"properties\":{\"mail\":" + std::string(SCHEMA_OBJ) + "}}"};
    return {};
}

static std::string input_schema_for(const std::string& resource, const std::string& operation) {
    return "{\"type\":\"object\",\"properties\":{}}";
}

static void enrich_descriptor(FunctionDescriptor& desc) {
    desc.tags = {desc.resource, desc.operation};
    desc.summary = desc.resource + " " + desc.operation;
    desc.description = "Demo function " + desc.id + " for " + desc.resource + " " + desc.operation + " action.";
    desc.operation_id = desc.id;
    const auto demo_schema = demo_schema_for(desc.id);
    if (!demo_schema.first.empty()) {
        desc.input_schema = demo_schema.first;
        desc.output_schema = demo_schema.second;
        return;
    }
    desc.input_schema = input_schema_for(desc.resource, desc.operation);
    desc.output_schema = R"({"type":"object","properties":{"status":{"type":"string"},"action":{"type":"string"}}})";
}

// Minimal JSON value extraction (no external deps)
static std::vector<std::string> extract_str_array(const std::string& json, const std::string& key) {
    std::vector<std::string> out;
    auto pos = json.find("\"" + key + "\"");
    if (pos == std::string::npos) return out;
    auto start = json.find('[', pos);
    if (start == std::string::npos) return out;
    auto end = json.find(']', start);
    if (end == std::string::npos) return out;
    std::string body = json.substr(start + 1, end - start - 1);
    std::string current;
    bool in_str = false;
    for (char c : body) {
        if (c == '"') { in_str = !in_str; continue; }
        if (c == ',' && !in_str) {
            if (!current.empty()) out.push_back(current);
            current.clear();
            continue;
        }
        if (in_str) current += c;
        else if (c != ' ' && c != '\n' && c != '\t' && c != '\r') current += c;
    }
    if (!current.empty()) out.push_back(current);
    return out;
}

static std::string extract_str(const std::string& json, const std::string& key) {
    auto pos = json.find("\"" + key + "\"");
    if (pos == std::string::npos) return "";
    pos = json.find(':', pos);
    if (pos == std::string::npos) return "";
    pos = json.find('"', pos + 1);
    if (pos == std::string::npos) return "";
    auto end = json.find('"', pos + 1);
    if (end == std::string::npos) return "";
    return json.substr(pos + 1, end - pos - 1);
}

static long long extract_int(const std::string& json, const std::string& key, long long def = 0) {
    auto pos = json.find("\"" + key + "\"");
    if (pos == std::string::npos) return def;
    pos = json.find(':', pos);
    if (pos == std::string::npos) return def;
    pos++;
    while (pos < json.size() && json[pos] == ' ') pos++;
    std::string num;
    while (pos < json.size() && (json[pos] >= '0' && json[pos] <= '9')) {
        num += json[pos++];
    }
    if (num.empty()) return def;
    return std::stoll(num);
}

static std::string resp(std::initializer_list<std::string> fields) {
    std::string r = "{";
    bool first = true;
    for (auto& f : fields) {
        if (!first) r += ",";
        first = false;
        r += f;
    }
    r += ",\"timestamp\":\"" + now_str() + "\"}";
    return r;
}

// ==================== Data Models ====================

struct PlayerRecord {
    std::string id, name, status, server, createdAt, updatedAt, last_login_at;
    int level = 1, vip = 0;
    long long gold = 0;
    std::string profile; // raw JSON

    std::string toJson() const {
        return "{" + json_str("id", id) + "," + json_str("name", name) +
               "," + json_int("level", level) + "," + json_int("vip", vip) +
               "," + json_int("gold", gold) + "," + json_str("status", status) +
               "," + json_str("server", server) + "," + json_str("createdAt", createdAt) +
               "," + json_str("updatedAt", updatedAt) + "," + json_str("last_login_at", last_login_at) +
               (profile.empty() ? "" : ",\"profile\":" + profile) + "}";
    }
};

struct OrderRecord {
    std::string id, playerId, productId, currency, status, channel, createdAt, updatedAt;
    long long amount = 0;
    std::string attributes;

    std::string toJson() const {
        return "{" + json_str("id", id) + "," + json_str("playerId", playerId) +
               "," + json_str("productId", productId) + "," + json_int("amount", amount) +
               "," + json_str("currency", currency) + "," + json_str("status", status) +
               "," + json_str("channel", channel) + "," + json_str("createdAt", createdAt) +
               "," + json_str("updatedAt", updatedAt) +
               (attributes.empty() ? "" : ",\"attributes\":" + attributes) + "}";
    }
};

struct LBEntry {
    std::string playerId, playerName, updatedAt;
    long long score = 0;
    int rank = 0;

    std::string toJson() const {
        return "{" + json_str("playerId", playerId) + "," + json_str("playerName", playerName) +
               "," + json_int("score", score) + "," + json_int("rank", rank) +
               "," + json_str("updatedAt", updatedAt) + "}";
    }
};

struct ItemRecord {
    std::string id, templateId, name, rarity, updatedAt;
    long long quantity = 0;

    std::string toJson() const {
        return "{" + json_str("id", id) + "," + json_str("templateId", templateId) +
               "," + json_str("name", name) + "," + json_int("quantity", quantity) +
               "," + json_str("rarity", rarity) + "," + json_str("updatedAt", updatedAt) + "}";
    }
};

struct MailRecord {
    std::string id, playerId, title, content, status, sentAt, updatedAt, expireAt;
    std::string reward;

    std::string toJson() const {
        return "{" + json_str("id", id) + "," + json_str("playerId", playerId) +
               "," + json_str("title", title) + "," + json_str("content", content) +
               "," + json_str("status", status) +
               (reward.empty() ? "" : ",\"reward\":" + reward) +
               "," + json_str("sentAt", sentAt) + "," + json_str("updatedAt", updatedAt) +
               (expireAt.empty() ? "" : "," + json_str("expireAt", expireAt)) + "}";
    }
};

// ==================== Store ====================

struct DemoStore {
    std::mutex mu;
    long long player_seq = 1002, order_seq = 3002, mail_seq = 5002;
    std::map<std::string, PlayerRecord> players;
    std::map<std::string, OrderRecord> orders;
    std::map<std::string, LBEntry> leaderboard;
    std::map<std::string, std::map<std::string, ItemRecord>> inventories;
    std::map<std::string, std::vector<MailRecord>> mails;

    DemoStore() {
        auto now = now_str();
        players["player_1001"] = {"player_1001", "Alice", "active", "s1", now, now, now, 35, 3, 128800,
            "{\"guild\":\"星海旅团\",\"country\":\"CN\",\"platform\":\"ios\"}"};
        players["player_1002"] = {"player_1002", "Bob", "active", "s2", now, now, now, 42, 5, 256000,
            "{\"guild\":\"苍穹守卫\",\"country\":\"US\",\"platform\":\"android\"}"};

        orders["order_3001"] = {"order_3001", "player_1001", "com.croupier.gems.648", "CNY", "paid", "appstore", now, now, 6480, "{\"region\":\"cn\"}"};
        orders["order_3002"] = {"order_3002", "player_1002", "battle.pass.s2", "USD", "pending", "googleplay", now, now, 68, ""};

        leaderboard["player_1002"] = {"player_1002", "Bob", now, 98500, 1};
        leaderboard["player_1001"] = {"player_1001", "Alice", now, 91200, 2};

        inventories["player_1001"]["gold_coin"] = {"item_gold_coin", "gold_coin", "金币", "common", now, 128800};
        inventories["player_1001"]["hero_ticket"] = {"item_hero_ticket", "hero_ticket", "英雄招募券", "rare", now, 12};

        mails["player_1001"] = {{"mail_5001", "player_1001", "开服奖励", "欢迎来到 Croupier Demo World", "unread",
            "{\"gold\":10000,\"item\":\"hero_ticket\"}", now, now, ""}};
    }
};

// ==================== Array JSON helper ====================

template<typename T>
static std::string arr_json(const std::map<std::string, T>& m) {
    std::string r = "[";
    bool first = true;
    for (auto& [k, v] : m) { if (!first) r += ","; first = false; r += v.toJson(); }
    return r + "]";
}

template<typename T>
static std::string arr_json_vec(const std::vector<T>& v) {
    std::string r = "[";
    bool first = true;
    for (auto& item : v) { if (!first) r += ","; first = false; r += item.toJson(); }
    return r + "]";
}

// ==================== Handler Registration ====================

static void registerAll(CroupierClient& client, DemoStore& store) {
    auto reg = [&](const std::string& id, const std::string& risk,
                   const std::string& resource, const std::string& op,
                   const std::string& capability, const std::string& execution,
                   FunctionHandler handler, const std::string& approval_policy = "") {
        FunctionDescriptor desc;
        desc.id = id; desc.version = "1.0.0"; desc.risk = risk;
        desc.resource = resource; desc.operation = op; desc.enabled = true;
        desc.capability = capability; desc.execution = execution;
        if (!approval_policy.empty()) {
            desc.approval_required = true;
            desc.approval_policy_key = approval_policy;
        }
        enrich_descriptor(desc);
        client.RegisterFunction(desc, handler);
        std::cout << "  registered: " << id << std::endl;
    };

    // player.create
    reg("player.create", "warning", "player", "create", "create", "sync",
        [&store](const std::string&, const std::string& payload) -> std::string {
            std::lock_guard<std::mutex> lk(store.mu);
            std::string id = extract_str(payload, "id");
            if (id.empty()) id = extract_str(payload, "playerId");
            if (id.empty()) id = "player_" + std::to_string(++store.player_seq);
            auto now = now_str();
            std::string name = extract_str(payload, "name");
            if (name.empty()) name = "Player-" + id;
            PlayerRecord r{id, name, "active", "s1", now, now, now,
                (int)extract_int(payload, "level", 1), (int)extract_int(payload, "vip", 0),
                extract_int(payload, "gold", 0), ""};
            store.players[id] = r;
            return resp({json_str("status", "success"), json_str("action", "player.create"),
                         "\"player\":" + r.toJson()});
        });

    // player.get
    reg("player.get", "safe", "player", "get", "item_query", "sync",
        [&store](const std::string&, const std::string& payload) -> std::string {
            std::lock_guard<std::mutex> lk(store.mu);
            std::string id = extract_str(payload, "playerId");
            if (id.empty()) id = extract_str(payload, "id");
            auto it = store.players.find(id);
            if (it == store.players.end())
                return resp({json_str("status", "not_found"), json_str("message", "player not found")});
            return resp({json_str("status", "success"), json_str("action", "player.get"),
                         "\"player\":" + it->second.toJson()});
        });

    // player.update
    reg("player.update", "warning", "player", "update", "update", "sync",
        [&store](const std::string&, const std::string& payload) -> std::string {
            std::lock_guard<std::mutex> lk(store.mu);
            std::string id = extract_str(payload, "playerId");
            if (id.empty()) id = extract_str(payload, "id");
            auto it = store.players.find(id);
            if (it == store.players.end())
                return resp({json_str("status", "not_found"), json_str("message", "player not found")});
            auto& r = it->second;
            auto n = extract_str(payload, "name"); if (!n.empty()) r.name = n;
            auto s = extract_str(payload, "status"); if (!s.empty()) r.status = s;
            auto sv = extract_str(payload, "server"); if (!sv.empty()) r.server = sv;
            if (payload.find("\"level\"") != std::string::npos) r.level = (int)extract_int(payload, "level", r.level);
            if (payload.find("\"vip\"") != std::string::npos) r.vip = (int)extract_int(payload, "vip", r.vip);
            if (payload.find("\"gold\"") != std::string::npos) r.gold = extract_int(payload, "gold", r.gold);
            r.updatedAt = now_str();
            return resp({json_str("status", "success"), json_str("action", "player.update"),
                         "\"player\":" + r.toJson()});
        });

    // player.delete
    reg("player.delete", "danger", "player", "delete", "delete", "sync",
        [&store](const std::string&, const std::string& payload) -> std::string {
            std::lock_guard<std::mutex> lk(store.mu);
            std::string id = extract_str(payload, "playerId");
            if (id.empty()) id = extract_str(payload, "id");
            store.players.erase(id); store.inventories.erase(id);
            store.mails.erase(id); store.leaderboard.erase(id);
            return resp({json_str("status", "success"), json_str("action", "player.delete"),
                         json_str("playerId", id)});
        },
        "player.delete.double_check");

    // player.list
    reg("player.list", "safe", "player", "list", "collection_query", "sync",
        [&store](const std::string&, const std::string&) -> std::string {
            std::lock_guard<std::mutex> lk(store.mu);
            return resp({json_str("status", "success"), json_str("action", "player.list"),
                         "\"items\":" + arr_json(store.players),
                         json_int("total", store.players.size())});
        });

    // order.create
    reg("order.create", "warning", "order", "create", "create", "sync",
        [&store](const std::string&, const std::string& payload) -> std::string {
            std::lock_guard<std::mutex> lk(store.mu);
            std::string id = extract_str(payload, "order_id");
            if (id.empty()) id = extract_str(payload, "id");
            if (id.empty()) id = "order_" + std::to_string(++store.order_seq);
            auto now = now_str();
            OrderRecord r{id, extract_str(payload, "playerId"),
                extract_str(payload, "productId").empty() ? "product.demo" : extract_str(payload, "productId"),
                extract_str(payload, "currency").empty() ? "CNY" : extract_str(payload, "currency"),
                extract_str(payload, "status").empty() ? "created" : extract_str(payload, "status"),
                extract_str(payload, "channel").empty() ? "gm" : extract_str(payload, "channel"),
                now, now, extract_int(payload, "amount", 0), ""};
            store.orders[id] = r;
            return resp({json_str("status", "success"), json_str("action", "order.create"),
                         "\"order\":" + r.toJson()});
        });

    // order.get
    reg("order.get", "safe", "order", "get", "item_query", "sync",
        [&store](const std::string&, const std::string& payload) -> std::string {
            std::lock_guard<std::mutex> lk(store.mu);
            std::string id = extract_str(payload, "order_id");
            if (id.empty()) id = extract_str(payload, "id");
            auto it = store.orders.find(id);
            if (it == store.orders.end())
                return resp({json_str("status", "not_found"), json_str("message", "order not found")});
            return resp({json_str("status", "success"), json_str("action", "order.get"),
                         "\"order\":" + it->second.toJson()});
        });

    // order.update
    reg("order.update", "warning", "order", "update", "update", "sync",
        [&store](const std::string&, const std::string& payload) -> std::string {
            std::lock_guard<std::mutex> lk(store.mu);
            std::string id = extract_str(payload, "order_id");
            if (id.empty()) id = extract_str(payload, "id");
            auto it = store.orders.find(id);
            if (it == store.orders.end())
                return resp({json_str("status", "not_found"), json_str("message", "order not found")});
            auto& r = it->second;
            auto s = extract_str(payload, "status"); if (!s.empty()) r.status = s;
            auto c = extract_str(payload, "channel"); if (!c.empty()) r.channel = c;
            if (payload.find("\"amount\"") != std::string::npos) r.amount = extract_int(payload, "amount", r.amount);
            r.updatedAt = now_str();
            return resp({json_str("status", "success"), json_str("action", "order.update"),
                         "\"order\":" + r.toJson()});
        });

    // order.delete
    reg("order.delete", "danger", "order", "delete", "delete", "sync",
        [&store](const std::string&, const std::string& payload) -> std::string {
            std::lock_guard<std::mutex> lk(store.mu);
            std::string id = extract_str(payload, "order_id");
            if (id.empty()) id = extract_str(payload, "id");
            store.orders.erase(id);
            return resp({json_str("status", "success"), json_str("action", "order.delete"),
                         json_str("order_id", id)});
        },
        "order.delete.double_check");

    // order.list
    reg("order.list", "safe", "order", "list", "collection_query", "sync",
        [&store](const std::string&, const std::string& payload) -> std::string {
            std::lock_guard<std::mutex> lk(store.mu);
            std::string pid = extract_str(payload, "playerId");
            if (pid.empty()) {
                return resp({json_str("status", "success"), json_str("action", "order.list"),
                             "\"items\":" + arr_json(store.orders),
                             json_int("total", store.orders.size())});
            }
            std::map<std::string, OrderRecord> filtered;
            for (auto& [k, v] : store.orders) if (v.playerId == pid) filtered[k] = v;
            return resp({json_str("status", "success"), json_str("action", "order.list"),
                         "\"items\":" + arr_json(filtered),
                         json_int("total", filtered.size())});
        });

    // leaderboard.list
    reg("leaderboard.list", "safe", "leaderboard", "list", "collection_query", "sync",
        [&store](const std::string&, const std::string&) -> std::string {
            std::lock_guard<std::mutex> lk(store.mu);
            // Sort by score descending
            std::vector<std::pair<std::string, LBEntry>> sorted(store.leaderboard.begin(), store.leaderboard.end());
            std::sort(sorted.begin(), sorted.end(), [](auto& a, auto& b) { return a.second.score > b.second.score; });
            std::string arr = "[";
            bool first = true;
            for (size_t i = 0; i < sorted.size(); i++) {
                auto e = sorted[i].second; e.rank = (int)(i + 1);
                if (!first) arr += ",";
                first = false;
                arr += e.toJson();
            }
            arr += "]";
            return resp({json_str("status", "success"), json_str("action", "leaderboard.list"),
                         "\"items\":" + arr, json_int("total", sorted.size())});
        });

    // leaderboard.upsert
    reg("leaderboard.upsert", "warning", "leaderboard", "upsert", "action", "sync",
        [&store](const std::string&, const std::string& payload) -> std::string {
            std::lock_guard<std::mutex> lk(store.mu);
            std::string pid = extract_str(payload, "playerId");
            if (pid.empty()) return resp({json_str("status", "error"), json_str("message", "playerId is required")});
            std::string pname = pid;
            auto pit = store.players.find(pid);
            if (pit != store.players.end() && !pit->second.name.empty()) pname = pit->second.name;
            LBEntry e{pid, pname, now_str(), extract_int(payload, "score", 0), 0};
            store.leaderboard[pid] = e;
            return resp({json_str("status", "success"), json_str("action", "leaderboard.upsert"),
                         "\"entry\":" + e.toJson()});
        });

    // leaderboard.reset
    reg("leaderboard.reset", "danger", "leaderboard", "reset", "action", "sync",
        [&store](const std::string&, const std::string&) -> std::string {
            std::lock_guard<std::mutex> lk(store.mu);
            store.leaderboard.clear();
            return resp({json_str("status", "success"), json_str("action", "leaderboard.reset")});
        },
        "leaderboard.reset.double_check");

    // inventory.list
    reg("inventory.list", "safe", "inventory", "list", "collection_query", "sync",
        [&store](const std::string&, const std::string& payload) -> std::string {
            std::lock_guard<std::mutex> lk(store.mu);
            std::string pid = extract_str(payload, "playerId");
            auto it = store.inventories.find(pid);
            if (it == store.inventories.end())
                return resp({json_str("status", "success"), json_str("action", "inventory.list"),
                             json_str("playerId", pid), "\"items\":[]"});
            return resp({json_str("status", "success"), json_str("action", "inventory.list"),
                         json_str("playerId", pid), "\"items\":" + arr_json(it->second)});
        });

    // inventory.grant
    reg("inventory.grant", "warning", "inventory", "grant", "action", "sync",
        [&store](const std::string&, const std::string& payload) -> std::string {
            std::lock_guard<std::mutex> lk(store.mu);
            std::string pid = extract_str(payload, "playerId");
            std::string tid = extract_str(payload, "templateId");
            if (tid.empty()) tid = extract_str(payload, "item_id");
            auto& inv = store.inventories[pid];
            auto it = inv.find(tid);
            if (it == inv.end()) {
                std::string name = extract_str(payload, "name");
                if (name.empty()) name = tid;
                std::string rarity = extract_str(payload, "rarity");
                if (rarity.empty()) rarity = "common";
                inv[tid] = {"item_" + tid, tid, name, rarity, "", 0};
                it = inv.find(tid);
            }
            it->second.quantity += extract_int(payload, "quantity", 1);
            it->second.updatedAt = now_str();
            return resp({json_str("status", "success"), json_str("action", "inventory.grant"),
                         json_str("playerId", pid), "\"item\":" + it->second.toJson()});
        });

    // inventory.consume
    reg("inventory.consume", "warning", "inventory", "consume", "action", "sync",
        [&store](const std::string&, const std::string& payload) -> std::string {
            std::lock_guard<std::mutex> lk(store.mu);
            std::string pid = extract_str(payload, "playerId");
            std::string tid = extract_str(payload, "templateId");
            if (tid.empty()) tid = extract_str(payload, "item_id");
            long long qty = extract_int(payload, "quantity", 1);
            auto iit = store.inventories.find(pid);
            if (iit == store.inventories.end())
                return resp({json_str("status", "not_found"), json_str("message", "item not found")});
            auto it = iit->second.find(tid);
            if (it == iit->second.end())
                return resp({json_str("status", "not_found"), json_str("message", "item not found")});
            if (it->second.quantity < qty)
                return resp({json_str("status", "failed"), json_str("message", "insufficient quantity")});
            it->second.quantity -= qty;
            it->second.updatedAt = now_str();
            return resp({json_str("status", "success"), json_str("action", "inventory.consume"),
                         json_str("playerId", pid), "\"item\":" + it->second.toJson()});
        });

    // mail.send
    reg("mail.send", "warning", "mail", "send", "action", "sync",
        [&store](const std::string&, const std::string& payload) -> std::string {
            std::lock_guard<std::mutex> lk(store.mu);
            std::string pid = extract_str(payload, "playerId");
            if (pid.empty()) return resp({json_str("status", "error"), json_str("message", "playerId is required")});
            auto now = now_str();
            std::string title = extract_str(payload, "title");
            if (title.empty()) title = "系统邮件";
            std::string content = extract_str(payload, "content");
            if (content.empty()) content = "请查收奖励";
            std::string mid = "mail_" + std::to_string(++store.mail_seq);
            MailRecord r{mid, pid, title, content, "unread", "", now, now, ""};
            // Extract reward as raw JSON substring
            auto rp = payload.find("\"reward\"");
            if (rp != std::string::npos) {
                auto start = payload.find('{', rp);
                if (start != std::string::npos) {
                    int depth = 0; size_t end = start;
                    for (size_t i = start; i < payload.size(); i++) {
                        if (payload[i] == '{') depth++;
                        if (payload[i] == '}') { depth--; if (depth == 0) { end = i; break; } }
                    }
                    r.reward = payload.substr(start, end - start + 1);
                }
            }
            store.mails[pid].push_back(r);
            return resp({json_str("status", "success"), json_str("action", "mail.send"),
                         "\"mail\":" + r.toJson()});
        });

    // mail.list
    reg("mail.list", "safe", "mail", "list", "collection_query", "sync",
        [&store](const std::string&, const std::string& payload) -> std::string {
            std::lock_guard<std::mutex> lk(store.mu);
            std::string pid = extract_str(payload, "playerId");
            auto it = store.mails.find(pid);
            if (it == store.mails.end())
                return resp({json_str("status", "success"), json_str("action", "mail.list"),
                             json_str("playerId", pid), "\"items\":[]", "\"total\":0"});
            return resp({json_str("status", "success"), json_str("action", "mail.list"),
                         json_str("playerId", pid), "\"items\":" + arr_json_vec(it->second),
                         json_int("total", it->second.size())});
        });

    // mail.claim
    reg("mail.claim", "warning", "mail", "claim", "action", "sync",
        [&store](const std::string&, const std::string& payload) -> std::string {
            std::lock_guard<std::mutex> lk(store.mu);
            std::string pid = extract_str(payload, "playerId");
            std::string mid = extract_str(payload, "mail_id");
            if (mid.empty()) mid = extract_str(payload, "id");
            auto it = store.mails.find(pid);
            if (it != store.mails.end()) {
                for (auto& m : it->second) {
                    if (m.id == mid) {
                        m.status = "claimed"; m.updatedAt = now_str();
                        return resp({json_str("status", "success"), json_str("action", "mail.claim"),
                                     "\"mail\":" + m.toJson()});
                    }
                }
            }
            return resp({json_str("status", "not_found"), json_str("message", "mail not found")});
        });

    // mail.batch_send: batch fan-out declared as a task-execution function
    reg("mail.batch_send", "high", "mail", "batch_send", "action", "task",
        [&store](const std::string&, const std::string& payload) -> std::string {
            std::lock_guard<std::mutex> lk(store.mu);
            auto ids = extract_str_array(payload, "playerIds");
            if (ids.empty()) return resp({json_str("status", "error"), json_str("message", "playerIds is required")});
            auto now = now_str();
            std::string title = extract_str(payload, "title");
            if (title.empty()) title = "系统邮件";
            std::string content = extract_str(payload, "content");
            if (content.empty()) content = "请查收奖励";
            long long seq_base = store.mail_seq;
            for (auto& pid : ids) {
                MailRecord r{"mail_" + std::to_string(++store.mail_seq), pid, title, content, "unread", "", now, now, ""};
                store.mails[pid].push_back(r);
            }
            return resp({json_str("status", "success"), json_str("action", "mail.batch_send"),
                         json_int("count", (long long)ids.size()),
                         json_int("firstMailSeq", seq_base + 1)});
        },
        "mail.batch_send.mass_send");
}

// ==================== Main ====================

int main() {
    std::signal(SIGINT, signalHandler);
    std::signal(SIGTERM, signalHandler);

    std::string agentAddr = "127.0.0.1:19091";
    std::string gameId = "demo-game";
    std::string serviceId = "game-demo-service";
    std::string envName = "development";

    if (auto v = std::getenv("CROUPIER_AGENT_ADDR")) agentAddr = v;
    if (auto v = std::getenv("CROUPIER_GAME_ID")) gameId = v;
    if (auto v = std::getenv("CROUPIER_SERVICE_ID")) serviceId = v;
    if (auto v = std::getenv("CROUPIER_ENV")) envName = v;

    std::cout << "starting game demo: agent=" << agentAddr
              << " game=" << gameId << " env=" << envName
              << " service=" << serviceId << std::endl;

    ClientConfig config;
    config.agent_addr = agentAddr;
    config.game_id = gameId;
    config.env = envName;
    config.service_id = serviceId;
    config.service_version = "1.0.0";
    config.insecure = true;
    config.auto_reconnect = true;

    CroupierClient client(config);
    DemoStore store;
    registerAll(client, store);

    std::cout << "connecting to agent..." << std::endl;
    if (client.Connect()) {
        std::cout << "connected, service started. press Ctrl+C to stop." << std::endl;
        while (!g_shutdown) {
            std::this_thread::sleep_for(std::chrono::milliseconds(500));
        }
    } else {
        std::cerr << "connection failed" << std::endl;
        return 1;
    }

    std::cout << "shutting down..." << std::endl;
    client.Close();
    return 0;
}
