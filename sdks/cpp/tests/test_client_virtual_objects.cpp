#include <gtest/gtest.h>
#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/config/client_config_loader.h"

using namespace croupier::sdk;
using namespace croupier::sdk::config;

// 测试夹具：虚拟对象测试
class ClientVirtualObjectsTest : public ::testing::Test {
protected:
    void SetUp() override {
        loader = std::make_unique<ClientConfigLoader>();
        config = loader->CreateDefaultConfig();
        client = std::make_unique<CroupierClient>(config);
    }

    void TearDown() override {
        // 清理
    }

    // 辅助函数：创建基本的虚拟对象描述符
    VirtualObjectDescriptor CreateBasicObjectDescriptor(const std::string& id) {
        VirtualObjectDescriptor desc;
        desc.id = id;
        desc.version = "1.0.0";
        desc.name = "Test Object";
        desc.description = "A test virtual object";
        return desc;
    }

    // 辅助函数：创建简单的函数处理器
    FunctionHandler CreateSimpleHandler(const std::string& response) {
        return [response](const std::string& context, const std::string& payload) -> std::string {
            return response;
        };
    }

    std::unique_ptr<ClientConfigLoader> loader;
    ClientConfig config;
    std::unique_ptr<CroupierClient> client;
};

// ========== 测试用例 ==========

TEST_F(ClientVirtualObjectsTest, RegisterSingleVirtualObject) {
    // RED: 测试注册单个虚拟对象
    VirtualObjectDescriptor desc = CreateBasicObjectDescriptor("test.entity");

    // 创建函数处理器映射
    std::map<std::string, FunctionHandler> handlers;
    handlers["create"] = CreateSimpleHandler("{\"result\":\"created\"}");
    handlers["read"] = CreateSimpleHandler("{\"result\":\"read\"}");

    // 注册虚拟对象
    bool success = client->RegisterVirtualObject(desc, handlers);

    // 注册应该成功
    EXPECT_TRUE(success);
}

TEST_F(ClientVirtualObjectsTest, RegisterMultipleVirtualObjects) {
    // RED: 测试注册多个虚拟对象
    // 创建第一个对象
    VirtualObjectDescriptor obj1 = CreateBasicObjectDescriptor("player.entity");
    std::map<std::string, FunctionHandler> handlers1;
    handlers1["create"] = CreateSimpleHandler("{\"result\":\"player created\"}");

    bool success1 = client->RegisterVirtualObject(obj1, handlers1);
    EXPECT_TRUE(success1);

    // 创建第二个对象
    VirtualObjectDescriptor obj2 = CreateBasicObjectDescriptor("item.entity");
    std::map<std::string, FunctionHandler> handlers2;
    handlers2["create"] = CreateSimpleHandler("{\"result\":\"item created\"}");

    bool success2 = client->RegisterVirtualObject(obj2, handlers2);
    EXPECT_TRUE(success2);

    // 创建第三个对象
    VirtualObjectDescriptor obj3 = CreateBasicObjectDescriptor("wallet.entity");
    std::map<std::string, FunctionHandler> handlers3;
    handlers3["read"] = CreateSimpleHandler("{\"result\":\"wallet read\"}");

    bool success3 = client->RegisterVirtualObject(obj3, handlers3);
    EXPECT_TRUE(success3);

    // 所有对象都应该成功注册
    EXPECT_TRUE(success1 && success2 && success3);
}

TEST_F(ClientVirtualObjectsTest, VirtualObjectDescriptorValidation) {
    // RED: 测试虚拟对象描述符验证
    VirtualObjectDescriptor desc = CreateBasicObjectDescriptor("validated.entity");

    // 验证必填字段
    EXPECT_FALSE(desc.id.empty());
    EXPECT_FALSE(desc.version.empty());
    EXPECT_FALSE(desc.name.empty());
    EXPECT_FALSE(desc.description.empty());

    // 验证字段值
    EXPECT_EQ(desc.id, "validated.entity");
    EXPECT_EQ(desc.version, "1.0.0");
    EXPECT_EQ(desc.name, "Test Object");
    EXPECT_EQ(desc.description, "A test virtual object");

    // 注册对象
    std::map<std::string, FunctionHandler> handlers;
    handlers["validate"] = CreateSimpleHandler("{\"result\":\"validated\"}");

    bool success = client->RegisterVirtualObject(desc, handlers);
    EXPECT_TRUE(success);
}

TEST_F(ClientVirtualObjectsTest, VirtualObjectWithHandlers) {
    // RED: 测试带函数处理器的虚拟对象
    VirtualObjectDescriptor desc = CreateBasicObjectDescriptor("handler.entity");

    // 创建多个函数处理器
    std::map<std::string, FunctionHandler> handlers;
    handlers["create"] = CreateSimpleHandler("{\"action\":\"create\"}");
    handlers["read"] = CreateSimpleHandler("{\"action\":\"read\"}");
    handlers["update"] = CreateSimpleHandler("{\"action\":\"update\"}");
    handlers["delete"] = CreateSimpleHandler("{\"action\":\"delete\"}");

    // 验证处理器数量
    EXPECT_EQ(handlers.size(), 4U);

    // 注册带处理器的虚拟对象
    bool success = client->RegisterVirtualObject(desc, handlers);
    EXPECT_TRUE(success);

    // 所有操作都应该有对应的处理器
    EXPECT_TRUE(handlers.find("create") != handlers.end());
    EXPECT_TRUE(handlers.find("read") != handlers.end());
    EXPECT_TRUE(handlers.find("update") != handlers.end());
    EXPECT_TRUE(handlers.find("delete") != handlers.end());
}

TEST_F(ClientVirtualObjectsTest, VirtualObjectWithRelationships) {
    // RED: 测试带关系的虚拟对象
    VirtualObjectDescriptor desc = CreateBasicObjectDescriptor("parent.entity");

    // 添加关系定义
    RelationshipDef rel1;
    rel1.type = "one-to-many";
    rel1.entity = "child.entity";
    rel1.foreign_key = "parent_id";

    RelationshipDef rel2;
    rel2.type = "many-to-one";
    rel2.entity = "owner.entity";
    rel2.foreign_key = "owner_id";

    desc.relationships["children"] = rel1;
    desc.relationships["owner"] = rel2;

    // 验证关系已添加
    EXPECT_EQ(desc.relationships.size(), 2U);
    EXPECT_EQ(desc.relationships["children"].type, "one-to-many");
    EXPECT_EQ(desc.relationships["children"].entity, "child.entity");
    EXPECT_EQ(desc.relationships["owner"].type, "many-to-one");
    EXPECT_EQ(desc.relationships["owner"].entity, "owner.entity");

    // 注册带关系的虚拟对象
    std::map<std::string, FunctionHandler> handlers;
    handlers["getChildren"] = CreateSimpleHandler("{\"children\":[]}");

    bool success = client->RegisterVirtualObject(desc, handlers);
    EXPECT_TRUE(success);
}

TEST_F(ClientVirtualObjectsTest, VirtualObjectWithSchema) {
    // RED: 测试带 schema 的虚拟对象
    VirtualObjectDescriptor desc = CreateBasicObjectDescriptor("schema.entity");

    // 添加 JSON Schema 定义
    desc.schema["id"] = "string";
    desc.schema["name"] = "string";
    desc.schema["level"] = "integer";
    desc.schema["score"] = "number";
    desc.schema["active"] = "boolean";

    // 验证 schema 字段
    EXPECT_EQ(desc.schema.size(), 5U);
    EXPECT_EQ(desc.schema["id"], "string");
    EXPECT_EQ(desc.schema["level"], "integer");
    EXPECT_EQ(desc.schema["active"], "boolean");

    // 添加操作映射
    desc.operations["create"] = "entity.create";
    desc.operations["read"] = "entity.get";
    desc.operations["update"] = "entity.update";

    // 验证操作映射
    EXPECT_EQ(desc.operations.size(), 3U);
    EXPECT_EQ(desc.operations["create"], "entity.create");

    // 添加元数据
    desc.metadata["category"] = "gameplay";
    desc.metadata["editable"] = "true";

    // 验证元数据
    EXPECT_EQ(desc.metadata.size(), 2U);
    EXPECT_EQ(desc.metadata["category"], "gameplay");

    // 注册带 schema 的虚拟对象
    // Note: handlers key must match operations value (function_id)
    std::map<std::string, FunctionHandler> handlers;
    handlers["entity.create"] = CreateSimpleHandler("{\"created\":true}");
    handlers["entity.get"] = CreateSimpleHandler("{\"data\":{}}");
    handlers["entity.update"] = CreateSimpleHandler("{\"updated\":true}");

    bool success = client->RegisterVirtualObject(desc, handlers);
    EXPECT_TRUE(success);
}

TEST_F(ClientVirtualObjectsTest, UnregisterVirtualObject) {
    // RED: 测试注销虚拟对象
    VirtualObjectDescriptor desc = CreateBasicObjectDescriptor("temp.entity");

    std::map<std::string, FunctionHandler> handlers;
    handlers["create"] = CreateSimpleHandler("{\"result\":\"created\"}");

    // 注册虚拟对象
    bool reg_success = client->RegisterVirtualObject(desc, handlers);
    EXPECT_TRUE(reg_success);

    // 注销虚拟对象
    bool unreg_success = client->UnregisterVirtualObject("temp.entity");
    EXPECT_TRUE(unreg_success);

    // 再次注销同一对象（应该失败或返回 false）
    bool unreg_fail = client->UnregisterVirtualObject("temp.entity");
    EXPECT_FALSE(unreg_fail);
}

TEST_F(ClientVirtualObjectsTest, GetRegisteredVirtualObjects) {
    // RED: 测试获取已注册的虚拟对象列表
    // 注册第一个对象
    VirtualObjectDescriptor obj1 = CreateBasicObjectDescriptor("object1.entity");
    std::map<std::string, FunctionHandler> handlers1;
    handlers1["read"] = CreateSimpleHandler("{\"result\":\"obj1\"}");
    client->RegisterVirtualObject(obj1, handlers1);

    // 注册第二个对象
    VirtualObjectDescriptor obj2 = CreateBasicObjectDescriptor("object2.entity");
    std::map<std::string, FunctionHandler> handlers2;
    handlers2["read"] = CreateSimpleHandler("{\"result\":\"obj2\"}");
    client->RegisterVirtualObject(obj2, handlers2);

    // 获取已注册的对象列表
    std::vector<VirtualObjectDescriptor> objects = client->GetRegisteredObjects();

    // 验证对象数量（至少应该有 2 个）
    EXPECT_GE(objects.size(), 2U);

    // 查找我们注册的对象
    bool found_obj1 = false;
    bool found_obj2 = false;

    for (const auto& obj : objects) {
        if (obj.id == "object1.entity") {
            found_obj1 = true;
            EXPECT_EQ(obj.name, "Test Object");
        }
        if (obj.id == "object2.entity") {
            found_obj2 = true;
            EXPECT_EQ(obj.version, "1.0.0");
        }
    }

    EXPECT_TRUE(found_obj1);
    EXPECT_TRUE(found_obj2);
}

TEST_F(ClientVirtualObjectsTest, InvalidVirtualObjectDescriptor) {
    // RED: 测试无效的虚拟对象描述符
    VirtualObjectDescriptor desc;

    // 不设置必填字段 id
    desc.id = "";
    desc.version = "1.0.0";
    desc.name = "Invalid Object";

    // 创建处理器
    std::map<std::string, FunctionHandler> handlers;
    handlers["create"] = CreateSimpleHandler("{\"result\":\"test\"}");

    // 尝试注册无效对象
    bool success = client->RegisterVirtualObject(desc, handlers);

    // 如果 id 为空，应该注册失败
    if (desc.id.empty()) {
        EXPECT_FALSE(success);
    }
}
