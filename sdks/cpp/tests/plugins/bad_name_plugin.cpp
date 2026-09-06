// Test-only plugin with an empty name: covers ValidatePlugin's name check.
#include "croupier/sdk/plugin/dynamic_loader.h"

using namespace croupier::sdk::plugin;

static PluginInfo bad_name_info = {
    "",
    "1.0.0",
    "SDK Tests",
    "Plugin with empty name",
    {"some_fn"},
    {},
};

extern "C" {

int croupier_plugin_init() { return 0; }

PluginInfo* croupier_plugin_info() { return &bad_name_info; }

void croupier_plugin_cleanup() {}

const char* some_fn(const char*, const char*) { return "{}"; }

}  // extern "C"
