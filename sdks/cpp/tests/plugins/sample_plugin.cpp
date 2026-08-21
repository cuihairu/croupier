// Test-only sample plugin used by unit tests to exercise the dynamic loader
// and plugin manager against a real shared object.
#include "croupier/sdk/plugin/dynamic_loader.h"

#include <string>

using namespace croupier::sdk::plugin;

static PluginInfo sample_info = {
    "sample_plugin",
    "2.1.0",
    "SDK Tests",
    "Plugin used by croupier-sdk-tests",
    {"sample_echo", "sample_missing"},
    {{"language", "C++"}},
};

static int g_init_calls = 0;

extern "C" {

int sample_plugin_init_calls() { return g_init_calls; }

int croupier_plugin_init() {
    ++g_init_calls;
    return 0;
}

PluginInfo* croupier_plugin_info() { return &sample_info; }

void croupier_plugin_cleanup() {}

const char* sample_echo(const char* context, const char* payload) {
    static std::string result;
    result = std::string("{\"echo\":true,\"context\":\"") + (context ? context : "") +
             "\",\"payload\":" + (payload ? payload : "null") + "}";
    return result.c_str();
}

}  // extern "C"
