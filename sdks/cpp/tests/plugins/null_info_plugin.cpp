// Test-only plugin whose croupier_plugin_info returns nullptr.
// Used to cover PluginManager::InitializePlugin's null-info failure path.
#include "croupier/sdk/plugin/dynamic_loader.h"

using croupier::sdk::plugin::PluginInfo;

extern "C" {

int croupier_plugin_init() { return 0; }

PluginInfo* croupier_plugin_info() { return nullptr; }

void croupier_plugin_cleanup() {}

}  // extern "C"
