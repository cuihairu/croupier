//go:build no_go_plugin

package config

import "fmt"

// validatePluginBuild fails closed in the trimmed build (todo.md:36
// 第二片): a binary built with the no_go_plugin tag contains no go-plugin
// host code, so declaring plugins is an operator mistake. Startup (config
// load) aborts with an explicit error instead of silently probing targets
// over plain HTTP while the config lies about plugin routing.
func validatePluginBuild(c Config) error {
	if len(c.Synthetics.Plugins) > 0 {
		return fmt.Errorf("synthetics.plugins declared but this binary was built without plugin support (no_go_plugin build tag); remove synthetics.plugins or rebuild without the tag")
	}
	return nil
}
