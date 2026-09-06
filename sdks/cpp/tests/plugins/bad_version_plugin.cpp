// Test-only plugin with an empty version: covers ValidatePlugin's version check.
#include "croupier/sdk/plugin/dynamic_loader.h"

using namespace croupier::sdk::plugin;

static PluginInfo bad_version_info = {
    "bad_version_plugin",
    "",
    "SDK Tests",
    "Plugin with empty version",
    {"some_fn"},
    {},
};

extern "C" {

int croupier_plugin_init() { return 0; }

PluginInfo* croupier_plugin_info() { return &bad_version_info; }

void croupier_plugin_cleanup() {}

const char* some_fn(const char*, const char*) { return "{}"; }

}  // extern "C"
