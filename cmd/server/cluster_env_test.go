package main

import (
	"testing"

	"github.com/cuihairu/croupier/internal/config"
)

// TestApplyClusterEnvironmentOverrides：HA 双实例部署共用同一份
// server.yaml，实例身份经环境变量注入（env 优先于 YAML）。
func TestApplyClusterEnvironmentOverrides(t *testing.T) {
	base := func() config.ClusterConfig {
		return config.ClusterConfig{
			Enabled:       false,
			InstanceID:    "yaml-id",
			AdvertiseAddr: "yaml:1",
		}
	}

	t.Run("no env keeps yaml values", func(t *testing.T) {
		c := &config.Config{Cluster: base()}
		applyClusterEnvironmentOverrides(c)
		if c.Cluster.InstanceID != "yaml-id" || c.Cluster.AdvertiseAddr != "yaml:1" || c.Cluster.Enabled {
			t.Fatalf("unexpected cluster config: %+v", c.Cluster)
		}
	})

	t.Run("env overrides yaml identity and enables", func(t *testing.T) {
		t.Setenv("CROUPIER_CLUSTER_ENABLED", "true")
		t.Setenv("CROUPIER_CLUSTER_INSTANCE_ID", "croupier-server2")
		t.Setenv("CROUPIER_CLUSTER_ADVERTISE_ADDR", "croupier-server2:19099")
		c := &config.Config{Cluster: base()}
		applyClusterEnvironmentOverrides(c)
		if !c.Cluster.Enabled {
			t.Fatal("expected enabled=true from env")
		}
		if c.Cluster.InstanceID != "croupier-server2" || c.Cluster.AdvertiseAddr != "croupier-server2:19099" {
			t.Fatalf("expected env override, got %+v", c.Cluster)
		}
	})

	t.Run("enabled truthy variants", func(t *testing.T) {
		for _, v := range []string{"1", "yes", "TRUE"} {
			t.Setenv("CROUPIER_CLUSTER_ENABLED", v)
			c := &config.Config{Cluster: base()}
			applyClusterEnvironmentOverrides(c)
			if !c.Cluster.Enabled {
				t.Fatalf("value %q should enable", v)
			}
		}
		t.Setenv("CROUPIER_CLUSTER_ENABLED", "false")
		c := &config.Config{Cluster: base()}
		applyClusterEnvironmentOverrides(c)
		if c.Cluster.Enabled {
			t.Fatal("'false' must not enable")
		}
	})
}

// 外部身份源凭据 env 覆盖（LDAP/OIDC secret 不落 yaml）。
func TestApplyAuthSecretEnvironmentOverrides(t *testing.T) {
	t.Run("no env keeps yaml", func(t *testing.T) {
		c := &config.Config{}
		applyAuthSecretEnvironmentOverrides(c)
		if c.Auth.Providers.LDAP.BindPassword != "" || c.Auth.Providers.OIDC.ClientSecret != "" {
			t.Fatalf("unexpected: %+v", c.Auth.Providers)
		}
	})
	t.Run("env overrides", func(t *testing.T) {
		t.Setenv("CROUPIER_AUTH_LDAP_BIND_PASSWORD", "ldapsecret")
		t.Setenv("CROUPIER_AUTH_OIDC_CLIENT_SECRET", "oidcsecret")
		c := &config.Config{}
		applyAuthSecretEnvironmentOverrides(c)
		if c.Auth.Providers.LDAP.BindPassword != "ldapsecret" {
			t.Fatal("ldap password not overridden")
		}
		if c.Auth.Providers.OIDC.ClientSecret != "oidcsecret" {
			t.Fatal("oidc secret not overridden")
		}
	})
}
