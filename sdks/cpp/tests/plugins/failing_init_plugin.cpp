// Test-only plugin whose croupier_plugin_init reports failure. Used to cover
// PluginManager::InitializePlugin failure branches.
#include "croupier/sdk/plugin/dynamic_loader.h"

using namespace croupier::sdk::plugin;

static PluginInfo failing_info = {
    "failing_init_plugin",
    "1.0.0",
    "SDK Tests",
    "Plugin whose init always fails",
    {},
    {},
};

extern "C" {

int croupier_plugin_init() { return 7; }

PluginInfo* croupier_plugin_info() { return &failing_info; }

void croupier_plugin_cleanup() {}

}  // extern "C"
