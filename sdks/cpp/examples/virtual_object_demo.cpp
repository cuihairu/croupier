#include "croupier/sdk/croupier_client.h"
#include <iostream>
#include <thread>
#include <chrono>

using namespace croupier::sdk;

// ========== Wallet Entity Handlers ==========

std::string WalletGetHandler(const std::string& /* context */, const std::string& payload) {
    std::cout << "WalletGet called with payload: " << payload << std::endl;

    auto data = utils::ParseJSON(payload);
    std::string wallet_id = data.count("wallet_id") ? data["wallet_id"] : "unknown";

    std::map<std::string, std::string> response;
    response["wallet_id"] = wallet_id;
    response["player_id"] = "player123";
    response["balance"] = "1000.50";
    response["currency"] = "gold";
    response["status"] = "active";

    return utils::ToJSON(response);
}

std::string WalletTransferHandler(const std::string& /* context */, const std::string& payload) {
    std::cout << "WalletTransfer called with payload: " << payload << std::endl;

    auto data = utils::ParseJSON(payload);
    std::string from_player = data["from_player_id"];
    std::string to_player = data["to_player_id"];
    std::string amount = data["amount"];

    // Simulate transfer logic
    std::cout << "Transferring " << amount << " from " << from_player << " to " << to_player << std::endl;

    std::map<std::string, std::string> response;
    response["transfer_id"] = "tx_" + utils::NewIdempotencyKey().substr(0, 8);
    response["status"] = "success";
    response["from_balance_after"] = "900.50";
    response["to_balance_after"] = "1100.50";
    response["processed_at"] = "2023-12-01T10:30:00Z";

    return utils::ToJSON(response);
}

std::string WalletDepositHandler(const std::string& /* context */, const std::string& payload) {
    std::cout << "WalletDeposit called with payload: " << payload << std::endl;

    auto data = utils::ParseJSON(payload);
    std::string player_id = data["player_id"];
    std::string amount = data["amount"];

    std::map<std::string, std::string> response;
    response["transaction_id"] = "dep_" + utils::NewIdempotencyKey().substr(0, 8);
    response["status"] = "success";
    response["new_balance"] = "1100.00";

    return utils::ToJSON(response);
}

// ========== Currency Entity Handlers ==========

std::string CurrencyGetHandler(const std::string& /* context */, const std::string& payload) {
    std::cout << "CurrencyGet called with payload: " << payload << std::endl;

    auto data = utils::ParseJSON(payload);
    std::string currency_id = data.count("currency_id") ? data["currency_id"] : "gold";

    std::map<std::string, std::string> response;
    response["currency_id"] = currency_id;
    response["name"] = "Gold Coins";
    response["symbol"] = "GOLD";
    response["exchange_rate"] = "1.0";
    response["enabled"] = "true";

    return utils::ToJSON(response);
}

std::string CurrencyCreateHandler(const std::string& /* context */, const std::string& payload) {
    std::cout << "CurrencyCreate called with payload: " << payload << std::endl;

    auto data = utils::ParseJSON(payload);
    std::string currency_name = data["name"];

    std::map<std::string, std::string> response;
    response["currency_id"] = "cur_" + utils::NewIdempotencyKey().substr(0, 8);
    response["name"] = currency_name;
    response["status"] = "created";

    return utils::ToJSON(response);
}

// ========== Demo Functions ==========

void DemoSingleFunction() {
    std::cout << "\n=== Demo 1: Single Function Registration ===" << std::endl;

    ClientConfig config;
    config.service_id = "demo-single-function";
    config.game_id = "mmorpg-demo";        // Required: Game identifier
    config.env = "development";            // Environment separation
    config.agent_addr = "127.0.0.1:19090"; // Agent address

    CroupierClient client(config);

    // Register single function (traditional way)
    FunctionDescriptor transfer_desc;
    transfer_desc.id = "wallet.transfer";
    transfer_desc.version = "1.0.0";

    if (client.RegisterFunction(transfer_desc, WalletTransferHandler)) {
        std::cout << "✅ Successfully registered single function: wallet.transfer" << std::endl;
    }

    // List registered objects (should be empty)
    auto objects = client.GetRegisteredObjects();
    std::cout << "📋 Registered objects: " << objects.size() << std::endl;
}

void DemoVirtualObject() {
    std::cout << "\n=== Demo 2: Virtual Object Registration ===" << std::endl;

    ClientConfig config;
    config.service_id = "demo-virtual-object";

    CroupierClient client(config);

    // Define wallet entity
    VirtualObjectDescriptor wallet_desc;
    wallet_desc.id = "wallet.entity";
    wallet_desc.version = "1.0.0";
    wallet_desc.name = "钱包实体";
    wallet_desc.description = "玩家钱包管理虚拟对象";

    // Define schema
    wallet_desc.schema["type"] = "object";
    wallet_desc.schema["properties"] = R"({
        "wallet_id": {"type": "string", "title": "钱包ID"},
        "player_id": {"type": "string", "title": "玩家ID"},
        "balance": {"type": "string", "pattern": "^[0-9]+\\.?[0-9]*$", "title": "余额"},
        "currency": {"type": "string", "title": "货币类型"}
    })";

    // Define operations
    wallet_desc.operations["read"] = "wallet.get";
    wallet_desc.operations["transfer"] = "wallet.transfer";
    wallet_desc.operations["deposit"] = "wallet.deposit";

    // Define relationships
    RelationshipDef currency_rel;
    currency_rel.type = "many-to-one";
    currency_rel.entity = "currency";
    currency_rel.foreign_key = "currency_id";
    wallet_desc.relationships["currency"] = currency_rel;

    // Prepare handlers
    std::map<std::string, FunctionHandler> wallet_handlers;
    wallet_handlers["wallet.get"] = WalletGetHandler;
    wallet_handlers["wallet.transfer"] = WalletTransferHandler;
    wallet_handlers["wallet.deposit"] = WalletDepositHandler;

    // Register virtual object
    if (client.RegisterVirtualObject(wallet_desc, wallet_handlers)) {
        std::cout << "✅ Successfully registered virtual object: " << wallet_desc.id << std::endl;
    }

    // List registered objects
    auto objects = client.GetRegisteredObjects();
    std::cout << "📋 Registered objects: " << objects.size() << std::endl;
    for (const auto& obj : objects) {
        std::cout << "  - " << obj.id << " (" << obj.name << ") with "
                  << obj.operations.size() << " operations" << std::endl;
    }
}

void DemoComponent() {
    std::cout << "\n=== Demo 3: Component Registration ===" << std::endl;

    ClientConfig config;
    config.service_id = "demo-economy-component";

    CroupierClient client(config);

    // Create wallet entity
    VirtualObjectDescriptor wallet_entity;
    wallet_entity.id = "wallet.entity";
    wallet_entity.version = "1.0.0";
    wallet_entity.name = "钱包实体";
    wallet_entity.description = "玩家钱包管理";
    wallet_entity.operations["read"] = "wallet.get";
    wallet_entity.operations["transfer"] = "wallet.transfer";
    wallet_entity.operations["deposit"] = "wallet.deposit";

    // Create currency entity
    VirtualObjectDescriptor currency_entity;
    currency_entity.id = "currency.entity";
    currency_entity.version = "1.0.0";
    currency_entity.name = "货币实体";
    currency_entity.description = "游戏货币管理";
    currency_entity.operations["read"] = "currency.get";
    currency_entity.operations["create"] = "currency.create";

    // Create economy component
    ComponentDescriptor economy_comp;
    economy_comp.id = "economy-system";
    economy_comp.version = "1.0.0";
    economy_comp.name = "经济系统组件";
    economy_comp.description = "完整的游戏经济管理系统";

    // Add entities to component
    economy_comp.entities.push_back(wallet_entity);
    economy_comp.entities.push_back(currency_entity);

    // Add standalone functions
    FunctionDescriptor market_trade;
    market_trade.id = "market.trade";
    market_trade.version = "1.0.0";
    economy_comp.functions.push_back(market_trade);

    // Add resource configuration
    economy_comp.resources["wallet_panel"] = "钱包管理面板";
    economy_comp.resources["currency_panel"] = "货币管理面板";

    // Register component
    if (client.RegisterComponent(economy_comp)) {
        std::cout << "✅ Successfully registered component: " << economy_comp.id << std::endl;
    }

    // List registered components and objects
    auto components = client.GetRegisteredComponents();
    auto objects = client.GetRegisteredObjects();

    std::cout << "📋 Registered components: " << components.size() << std::endl;
    for (const auto& comp : components) {
        std::cout << "  - " << comp.id << " (" << comp.name << ") with "
                  << comp.entities.size() << " entities" << std::endl;
    }

    std::cout << "📋 Registered objects: " << objects.size() << std::endl;
    for (const auto& obj : objects) {
        std::cout << "  - " << obj.id << " (" << obj.name << ")" << std::endl;
    }
}

void DemoTemplateGeneration() {
    std::cout << "\n=== Demo 4: Template Generation ===" << std::endl;

    // Generate object template
    std::string object_template = utils::GenerateObjectTemplate("player");
    std::cout << "🎨 Generated object template for 'player':" << std::endl;
    std::cout << object_template << std::endl;

    std::cout << "\n" << std::string(50, '-') << std::endl;

    // Generate component template
    std::string component_template = utils::GenerateComponentTemplate("player-system");
    std::cout << "🎨 Generated component template for 'player-system':" << std::endl;
    std::cout << component_template << std::endl;
}

void DemoSerialization() {
    std::cout << "\n=== Demo 5: Serialization ===" << std::endl;

    // Create a sample virtual object
    VirtualObjectDescriptor sample_obj;
    sample_obj.id = "sample.entity";
    sample_obj.version = "1.0.0";
    sample_obj.name = "示例实体";
    sample_obj.description = "这是一个序列化演示实体";
    sample_obj.operations["read"] = "sample.get";
    sample_obj.operations["create"] = "sample.create";

    RelationshipDef sample_rel;
    sample_rel.type = "one-to-many";
    sample_rel.entity = "related_entity";
    sample_rel.foreign_key = "sample_id";
    sample_obj.relationships["children"] = sample_rel;

    // Serialize to JSON
    std::string json = utils::ObjectDescriptorToJSON(sample_obj);
    std::cout << "🔄 Serialized virtual object to JSON:" << std::endl;
    std::cout << json << std::endl;
}

void DemoValidation() {
    std::cout << "\n=== Demo 6: Validation ===" << std::endl;

    // Valid object
    VirtualObjectDescriptor valid_obj;
    valid_obj.id = "valid.entity";
    valid_obj.version = "1.0.0";
    valid_obj.operations["read"] = "valid.get";

    bool is_valid = utils::ValidateObjectDescriptor(valid_obj);
    std::cout << "✅ Valid object validation result: " << (is_valid ? "PASS" : "FAIL") << std::endl;

    // Invalid object (missing operations)
    VirtualObjectDescriptor invalid_obj;
    invalid_obj.id = "invalid.entity";
    invalid_obj.version = "1.0.0";
    // No operations defined!

    bool is_invalid = utils::ValidateObjectDescriptor(invalid_obj);
    std::cout << "❌ Invalid object validation result: " << (is_invalid ? "PASS" : "FAIL") << std::endl;
}

int main() {
    std::cout << "🚀 Croupier C++ SDK Virtual Object Registration Demo" << std::endl;
    std::cout << "======================================================" << std::endl;

    try {
        // Run all demos
        DemoSingleFunction();
        DemoVirtualObject();
        DemoComponent();
        DemoTemplateGeneration();
        DemoSerialization();
        DemoValidation();

        std::cout << "\n🎉 All demos completed successfully!" << std::endl;
        std::cout << "\n📚 Next steps:" << std::endl;
        std::cout << "  1. Implement real gRPC integration" << std::endl;
        std::cout << "  2. Add JSON file loading support" << std::endl;
        std::cout << "  3. Enhance schema validation" << std::endl;
        std::cout << "  4. Add TLS/mTLS security" << std::endl;

    } catch (const std::exception& e) {
        std::cerr << "❌ Demo failed with error: " << e.what() << std::endl;
        return 1;
    }

    return 0;
}