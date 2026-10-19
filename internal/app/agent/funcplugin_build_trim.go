//go:build no_go_plugin

package agent

import (
	"fmt"

	"github.com/cuihairu/croupier/agents/sidecar/funcplugin"
)

// startFunctionPlugins fails closed in the trimmed build (todo.md:35,
// same 口径 as the synthetics plugingate): a binary built with the
// no_go_plugin tag contains no go-plugin host code, so declaring
// function plugins is an operator mistake — startup aborts with an
// explicit error instead of silently dropping the declared functions.
func (a *App) startFunctionPlugins(plugins []funcplugin.PluginConfig) error {
	if len(plugins) > 0 {
		return fmt.Errorf("functions.plugins declared but this binary was built without plugin support (no_go_plugin build tag); remove functions.plugins or rebuild without the tag")
	}
	return nil
}
