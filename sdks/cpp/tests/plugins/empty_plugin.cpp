// Test-only plugin whose info is valid but declares no provided functions.
// Used to cover PluginManager::ValidatePlugin's "no functions" warning path.
#include "croupier/sdk/plugin/dynamic_loader.h"

using namespace croupier::sdk::plugin;

static PluginInfo empty_info = {
    "empty_plugin",
    "1.0.0",
    "SDK Tests",
    "Plugin without provided functions",
    {},
    {},
};

extern "C" {

int croupier_plugin_init() { return 0; }

PluginInfo* croupier_plugin_info() { return &empty_info; }

void croupier_plugin_cleanup() {}

}  // extern "C"
