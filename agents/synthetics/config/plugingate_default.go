//go:build !no_go_plugin

package config

// validatePluginBuild is a no-op in the default build: the go-plugin
// host is compiled in (默认集成) and the switch/trim semantics live in
// the runner and pluginhost packages.
func validatePluginBuild(Config) error { return nil }
