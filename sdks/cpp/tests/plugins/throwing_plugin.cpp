// Test-only plugin whose exported function throws and whose cleanup throws.
// Covers GetPluginFunction's handler exception path and CleanupPlugin's
// cleanup exception path.
#include "croupier/sdk/plugin/dynamic_loader.h"

#include <stdexcept>
#include <string>

using namespace croupier::sdk::plugin;

static PluginInfo throwing_info = {
    "throwing_plugin",
    "1.0.0",
    "SDK Tests",
    "Plugin whose function and cleanup throw",
    {"throwing_fn"},
    {},
};

extern "C" {

int croupier_plugin_init() { return 0; }

PluginInfo* croupier_plugin_info() { return &throwing_info; }

void croupier_plugin_cleanup() { throw std::runtime_error("cleanup boom"); }

const char* throwing_fn(const char*, const char*) { throw std::runtime_error("fn boom"); }

}  // extern "C"
