#include <gtest/gtest.h>
#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/config/client_config_loader.h"
#include <fstream>

using namespace croupier::sdk;
using namespace croupier::sdk::config;

// 测试夹具：组件注册测试
class ClientComponentsTest : public ::testing::Test {
protected:
    void SetUp() override {
        loader = std::make_unique<ClientConfigLoader>();
        config = loader->CreateDefaultConfig();
        client = std::make_unique<CroupierClient>(config);
    }

    void TearDown() override {
        // 清理测试文件
        for (const auto& file : test_files) {
            std::remove(file.c_str());
        }
        test_files.clear();
    }

    // 辅助函数：创建基本的组件描述符
    ComponentDescriptor CreateBasicComponentDescriptor(const std::string& id) {
        ComponentDescriptor desc;
        desc.id = id;
        desc.version = "1.0.0";
        desc.name = "Test Component";
        desc.description = "A test component";
        desc.type = "gameplay";
        return desc;
    }

    // 辅助函数：创建临时测试文件
    std::string CreateTempFile(const std::string& filename, const std::string& content) {
        std::ofstream file(filename);
        file << content;
        file.close();
        test_files.push_back(filename);
        return filename;
    }

    std::unique_ptr<ClientConfigLoader> loader;
    ClientConfig config;
    std::unique_ptr<CroupierClient> client;
    std::vector<std::string> test_files;
};

// ========== 测试用例 ==========

TEST_F(ClientComponentsTest, RegisterSingleComponent) {
    // RED: 测试注册单个组件
    ComponentDescriptor desc = CreateBasicComponentDescriptor("test-component");

    // 添加元数据
    desc.metadata["category"] = "economy";
    desc.metadata["editable"] = "true";

    // 验证描述符
    EXPECT_EQ(desc.id, "test-component");
    EXPECT_EQ(desc.version, "1.0.0");
    EXPECT_EQ(desc.name, "Test Component");
    EXPECT_TRUE(desc.enabled);

    // 注册组件
    bool success = client->RegisterComponent(desc);

    // 注册应该成功
    EXPECT_TRUE(success);
}

TEST_F(ClientComponentsTest, RegisterComponentWithEntities) {
    // RED: 测试组件包含多个实体
    ComponentDescriptor desc = CreateBasicComponentDescriptor("entity-component");

    // 创建第一个虚拟对象
    VirtualObjectDescriptor entity1;
    entity1.id = "player.entity";
    entity1.version = "1.0.0";
    entity1.name = "Player Entity";
    entity1.description = "Player virtual object";
    desc.entities.push_back(entity1);

    // 创建第二个虚拟对象
    VirtualObjectDescriptor entity2;
    entity2.id = "item.entity";
    entity2.version = "1.0.0";
    entity2.name = "Item Entity";
    entity2.description = "Item virtual object";
    desc.entities.push_back(entity2);

    // 创建第三个虚拟对象
    VirtualObjectDescriptor entity3;
    entity3.id = "wallet.entity";
    entity3.version = "1.0.0";
    entity3.name = "Wallet Entity";
    entity3.description = "Wallet virtual object";
    desc.entities.push_back(entity3);

    // 验证实体数量
    EXPECT_EQ(desc.entities.size(), 3U);

    // 注册组件
    bool success = client->RegisterComponent(desc);
    EXPECT_TRUE(success);
}

TEST_F(ClientComponentsTest, RegisterComponentWithFunctions) {
    // RED: 测试组件包含多个函数
    ComponentDescriptor desc = CreateBasicComponentDescriptor("function-component");

    // 创建函数描述符
    FunctionDescriptor func1;
    func1.id = "player.create";
    func1.version = "1.0.0";
    func1.category = "player";
    func1.risk = "low";
    func1.entity = "player";
    func1.operation = "create";
    desc.functions.push_back(func1);

    FunctionDescriptor func2;
    func2.id = "player.update";
    func2.version = "1.0.0";
    func2.category = "player";
    func2.risk = "medium";
    func2.entity = "player";
    func2.operation = "update";
    desc.functions.push_back(func2);

    FunctionDescriptor func3;
    func3.id = "player.delete";
    func3.version = "1.0.0";
    func3.category = "player";
    func3.risk = "high";
    func3.entity = "player";
    func3.operation = "delete";
    desc.functions.push_back(func3);

    // 验证函数数量
    EXPECT_EQ(desc.functions.size(), 3U);

    // 验证风险等级
    EXPECT_EQ(desc.functions[0].risk, "low");
    EXPECT_EQ(desc.functions[1].risk, "medium");
    EXPECT_EQ(desc.functions[2].risk, "high");

    // 注册组件
    bool success = client->RegisterComponent(desc);
    EXPECT_TRUE(success);
}

TEST_F(ClientComponentsTest, ComponentDescriptorValidation) {
    // RED: 测试组件描述符验证
    ComponentDescriptor desc = CreateBasicComponentDescriptor("validated-component");

    // 验证必填字段
    EXPECT_FALSE(desc.id.empty());
    EXPECT_FALSE(desc.version.empty());
    EXPECT_FALSE(desc.name.empty());
    EXPECT_FALSE(desc.description.empty());
    EXPECT_FALSE(desc.type.empty());

    // 添加资源配置
    desc.resources["ui_path"] = "/ui/components/test";
    desc.resources["icon"] = "/icons/test.png";

    // 添加配置
    desc.config["max_items"] = "100";
    desc.config["auto_save"] = "true";

    // 注册组件
    bool success = client->RegisterComponent(desc);
    EXPECT_TRUE(success);
}

TEST_F(ClientComponentsTest, ComponentWithDependencies) {
    // RED: 测试组件依赖关系
    ComponentDescriptor desc = CreateBasicComponentDescriptor("dependent-component");

    // 添加依赖项
    desc.dependencies.push_back("database-module");
    desc.dependencies.push_back("auth-service");
    desc.dependencies.push_back("cache-provider");

    // 验证依赖数量
    EXPECT_EQ(desc.dependencies.size(), 3U);

    // 验证依赖项
    EXPECT_TRUE(desc.dependencies[0] == "database-module");
    EXPECT_TRUE(desc.dependencies[1] == "auth-service");
    EXPECT_TRUE(desc.dependencies[2] == "cache-provider");

    // 注册组件
    bool success = client->RegisterComponent(desc);
    EXPECT_TRUE(success);
}

TEST_F(ClientComponentsTest, LoadComponentFromFile) {
    // RED: 测试从文件加载组件
    std::string component_json = R"({
        "id": "file-component",
        "version": "1.0.0",
        "name": "File Component",
        "description": "Component loaded from file",
        "type": "gameplay",
        "enabled": true,
        "entities": [
            {
                "id": "file.entity",
                "version": "1.0.0",
                "name": "File Entity",
                "description": "Entity from file"
            }
        ],
        "functions": [
            {
                "id": "file.function",
                "version": "1.0.0",
                "category": "test",
                "risk": "low",
                "entity": "file",
                "operation": "create",
                "enabled": true
            }
        ],
        "config": {
            "setting1": "value1",
            "setting2": "value2"
        },
        "metadata": {
            "author": "test",
            "license": "MIT"
        }
    })";

    std::string component_file = CreateTempFile("test_component.json", component_json);

    // 从文件加载组件
    bool success = client->LoadComponentFromFile(component_file);

    // 加载应该成功
    EXPECT_TRUE(success);
}

TEST_F(ClientComponentsTest, UnregisterComponent) {
    // RED: 测试注销组件
    ComponentDescriptor desc = CreateBasicComponentDescriptor("temp-component");

    // 添加一些元数据
    desc.metadata["temporary"] = "true";

    // 注册组件
    bool reg_success = client->RegisterComponent(desc);
    EXPECT_TRUE(reg_success);

    // 注销组件
    bool unreg_success = client->UnregisterComponent("temp-component");
    EXPECT_TRUE(unreg_success);

    // 再次注销同一组件（应该失败或返回 false）
    bool unreg_fail = client->UnregisterComponent("temp-component");
    EXPECT_FALSE(unreg_fail);
}

TEST_F(ClientComponentsTest, GetRegisteredComponents) {
    // RED: 测试获取已注册的组件列表
    // 注册第一个组件
    ComponentDescriptor comp1 = CreateBasicComponentDescriptor("component1");
    comp1.type = "gameplay";
    client->RegisterComponent(comp1);

    // 注册第二个组件
    ComponentDescriptor comp2 = CreateBasicComponentDescriptor("component2");
    comp2.type = "economy";
    client->RegisterComponent(comp2);

    // 注册第三个组件
    ComponentDescriptor comp3 = CreateBasicComponentDescriptor("component3");
    comp3.type = "social";
    client->RegisterComponent(comp3);

    // 获取已注册的组件列表
    std::vector<ComponentDescriptor> components = client->GetRegisteredComponents();

    // 验证组件数量（至少应该有 3 个）
    EXPECT_GE(components.size(), 3U);

    // 查找我们注册的组件
    bool found_comp1 = false;
    bool found_comp2 = false;
    bool found_comp3 = false;

    for (const auto& comp : components) {
        if (comp.id == "component1") {
            found_comp1 = true;
            EXPECT_EQ(comp.type, "gameplay");
        }
        if (comp.id == "component2") {
            found_comp2 = true;
            EXPECT_EQ(comp.type, "economy");
        }
        if (comp.id == "component3") {
            found_comp3 = true;
            EXPECT_EQ(comp.type, "social");
        }
    }

    EXPECT_TRUE(found_comp1);
    EXPECT_TRUE(found_comp2);
    EXPECT_TRUE(found_comp3);
}

TEST_F(ClientComponentsTest, InvalidComponentDescriptor) {
    // RED: 测试无效的组件描述符
    ComponentDescriptor desc;

    // 不设置必填字段 id
    desc.id = "";
    desc.version = "1.0.0";
    desc.name = "Invalid Component";
    desc.type = "test";

    // 尝试注册无效组件
    bool success = client->RegisterComponent(desc);

    // 如果 id 为空，应该注册失败
    if (desc.id.empty()) {
        EXPECT_FALSE(success);
    }
}
