package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestUnmarshalAuthProviders_CanonicalKeys(t *testing.T) {
	input := `
auth:
  jwtSecret: s3cret
  providers:
    ldap:
      enabled: true
      addr: ldap://ldap.example.com:389
      baseDn: dc=example,dc=com
      bindDn: uid=svc,ou=system,dc=example,dc=com
      bindPassword: pw
      userFilter: "(uid=%s)"
      userDnTemplate: "uid=%s,ou=people,dc=example,dc=com"
      startTls: true
      insecureSkipVerify: false
      defaultRoles: [viewer, ops]
    oidc:
      enabled: true
      issuer: https://idp.example.com
      clientId: croupier
      clientSecret: cs
      redirectUrl: http://localhost:18780/api/auth/oidc/callback
      scopes: [openid, profile, email]
      usernameClaim: preferred_username
      defaultRoles: [viewer]
      loginSuccessUrl: http://localhost:8000/login
`

	var cfg Config
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	ldap := cfg.Auth.Providers.LDAP
	if !ldap.Enabled || ldap.Addr != "ldap://ldap.example.com:389" || ldap.BaseDN != "dc=example,dc=com" {
		t.Fatalf("ldap base fields mismatch: %+v", ldap)
	}
	if ldap.BindDN != "uid=svc,ou=system,dc=example,dc=com" || ldap.BindPassword != "pw" {
		t.Fatalf("ldap bind mismatch: %+v", ldap)
	}
	if ldap.UserFilter != "(uid=%s)" || ldap.UserDNTemplate != "uid=%s,ou=people,dc=example,dc=com" {
		t.Fatalf("ldap filter/template mismatch: %+v", ldap)
	}
	if !ldap.StartTLS || ldap.InsecureSkipVerify {
		t.Fatalf("ldap tls flags mismatch: %+v", ldap)
	}
	if len(ldap.DefaultRoles) != 2 || ldap.DefaultRoles[0] != "viewer" || ldap.DefaultRoles[1] != "ops" {
		t.Fatalf("ldap defaultRoles mismatch: %+v", ldap.DefaultRoles)
	}

	oidc := cfg.Auth.Providers.OIDC
	if !oidc.Enabled || oidc.Issuer != "https://idp.example.com" || oidc.ClientID != "croupier" || oidc.ClientSecret != "cs" {
		t.Fatalf("oidc base fields mismatch: %+v", oidc)
	}
	if oidc.RedirectURL != "http://localhost:18780/api/auth/oidc/callback" {
		t.Fatalf("oidc redirectUrl mismatch: %+v", oidc)
	}
	if oidc.UsernameClaim != "preferred_username" || oidc.LoginSuccessURL != "http://localhost:8000/login" {
		t.Fatalf("oidc claim/success url mismatch: %+v", oidc)
	}
	if len(oidc.Scopes) != 3 {
		t.Fatalf("oidc scopes mismatch: %+v", oidc.Scopes)
	}
}

func TestUnmarshalAuthProviders_DisabledByDefault(t *testing.T) {
	input := `
auth:
  jwtSecret: s3cret
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	if cfg.Auth.Providers.LDAP.Enabled || cfg.Auth.Providers.OIDC.Enabled {
		t.Fatal("providers must be disabled by default")
	}
}
