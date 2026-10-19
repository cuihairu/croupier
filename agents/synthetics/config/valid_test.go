// Shared test fixture for synthetics-agent config validation, available
// to both the default and the no_go_plugin trimmed build.
package config

// valid is the minimal passing config fixture.
func valid() Config {
	return Config{Agent: Agent{ID: "a", ServerAddr: "s", GameID: "g", Env: "e"}, Synthetics: Synthetics{Interval: 1, Targets: []Target{{Name: "t", URL: "http://127.0.0.1", Method: "GET", Timeout: 1, ExpectedStatus: 200}}}}
}
