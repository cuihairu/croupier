package croupier

import (
	"testing"
)

// AUD-102: config sources must be able to express an explicit secure
// posture. A bool alone cannot distinguish "omitted" from "explicitly false",
// so the mode selector carries the intent and the legacy bool stays intact
// for existing callers.

func TestAUD102_ResolveInsecureMode(t *testing.T) {
	cases := []struct {
		name       string
		insecure   bool
		mode       string
		tlsSet     bool
		fallback   bool
		wantSecure bool
	}{
		{"omitted with no TLS inherits default", false, InsecureModeUnset, false, true, false},
		{"omitted with CA is secure", false, InsecureModeUnset, true, true, true},
		{"explicit insecure wins over CA", true, InsecureModeUnset, true, true, false},
		{"explicit secure mode", false, InsecureModeSecure, true, true, true},
		{"explicit insecure mode", false, InsecureModeInsecure, true, true, false},
		{"secure mode beats legacy bool", true, InsecureModeSecure, true, true, true},
		{"insecure mode beats legacy false", false, InsecureModeInsecure, false, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveInsecureMode(tc.insecure, tc.mode, tc.tlsSet, tc.fallback)
			if got != !tc.wantSecure {
				t.Fatalf("resolveInsecureMode(...) insecure=%v, want secure=%v", got, tc.wantSecure)
			}
		})
	}
}

func TestAUD102_MergeWithDefaultsExplicitSecure(t *testing.T) {
	merged := MergeWithDefaults(&ClientConfig{
		AgentAddr: "agent.example:19091",
		CAFile:    "/etc/croupier/ca.pem",
	})
	if merged.Insecure {
		t.Fatal("explicit CA with no insecure flag must merge to secure TLS")
	}
	if merged.InsecureMode != InsecureModeSecure {
		t.Fatalf("InsecureMode = %q, want %q", merged.InsecureMode, InsecureModeSecure)
	}
	if merged.CAFile != "/etc/croupier/ca.pem" {
		t.Fatalf("CAFile lost: %q", merged.CAFile)
	}
	if err := ValidateClientConfig(merged); err == nil {
		t.Fatal("merged secure config must validate when CA is present")
	}

	explicit := MergeWithDefaults(&ClientConfig{
		Insecure:  true,
		CAFile:    "/etc/croupier/ca.pem",
		AgentAddr: "agent.example:19091",
	})
	if !explicit.Insecure || explicit.InsecureMode != InsecureModeInsecure {
		t.Fatalf("explicit insecure flag must survive merge: %+v", explicit)
	}

	defaulted := MergeWithDefaults(&ClientConfig{AgentAddr: "agent.example:19091"})
	if !defaulted.Insecure || defaulted.InsecureMode != InsecureModeInsecure {
		t.Fatalf("fully omitted security fields must inherit development default: %+v", defaulted)
	}
}

func TestAUD102_NewClientCarriesMergedTLS(t *testing.T) {
	c := NewClient(&ClientConfig{
		AgentAddr:  "agent.example:19091",
		CAFile:     "/etc/croupier/ca.pem",
		ServerName: "agent.example",
	})
	impl, ok := c.(*client)
	if !ok {
		t.Fatalf("NewClient returned %T, want *client", c)
	}
	if impl.config.Insecure {
		t.Fatal("NewClient did not resolve explicit CA config to secure TLS")
	}
	managerConfig := impl.buildManagerConfig()
	if managerConfig.Insecure {
		t.Fatal("buildManagerConfig did not carry resolved secure posture to transport")
	}
	if managerConfig.CAFile != "/etc/croupier/ca.pem" || managerConfig.ServerName != "agent.example" {
		t.Fatalf("TLS options lost at transport boundary: %+v", managerConfig)
	}

	legacy := NewClient(&ClientConfig{AgentAddr: "agent.example:19091", Insecure: true})
	implLegacy := legacy.(*client)
	if !implLegacy.buildManagerConfig().Insecure {
		t.Fatal("legacy explicit insecure=true must remain insecure")
	}
}
